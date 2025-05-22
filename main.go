package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/skip2/go-qrcode"
)

type Output string

const (
	OutputBase64 Output = "base64"
	OutputPNG    Output = "png"
)

type QRRequest struct {
	ID         string `json:"id"`
	Data       string `json:"data"`                  // required
	Size       int    `json:"size"`                  // optional (default 256)
	FGColor    string `json:"fg_color"`              // optional (default #000000)
	BGColor    string `json:"bg_color"`              // optional (default #ffffff)
	LogoBase64 string `json:"logo_base64,omitempty"` // optional
	Output     Output `json:"output"`                // "png" or "base64"
}
type BatchPayload struct {
	QRRequests []QRRequest `json:"qr_requests"`
	BatchID    string      `json:"batch_id"`
}

type Response struct {
	Message string `json:"message"`
}

type Base64Response struct {
	ImageBase64 string `json:"image_base64"`
}

var redisClient *redis.Client

func main() {
	initRedisClient()

	e := echo.New()
	e.POST("/generate", generateCustomQR)
	e.POST("/generateBase64", generateBase64FromImage)
	e.POST("/generatebatch", generateBatchQR)
	e.GET("/qr/:id", getQRResult)
	e.POST("/batch-results", getBatchResults)

	go qrWorker()
	log.Fatal(e.Start(":8080"))
}

type BatchResultRequest struct {
	IDs []string `json:"ids"`
}
type BatchResultItem struct {
	ID     string `json:id"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func initRedisClient() error {

	redisClient = redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return err
	}
	return nil
}
func getBatchResults(c echo.Context) error {
	var req BatchResultRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid input"})
	}
	ctx := context.Background()
	results := make([]BatchResultItem, 0, len(req.IDs))
	for _, id := range req.IDs {
		key := "qr_result:" + id
		result, err := redisClient.Get(ctx, key).Result()
		if err == redis.Nil {
			results = append(results, BatchResultItem{
				ID:    id,
				Error: "result not ready ",
			})
		} else if err != nil {
			results = append(results, BatchResultItem{
				ID:    id,
				Error: err.Error(),
			})
		} else {
			results = append(results, BatchResultItem{
				ID:     id,
				Result: result,
			})
		}
	}

	return c.JSON(http.StatusOK, results)
}
func generateBatchQR(c echo.Context) error {
	var payloads BatchPayload
	if err := c.Bind(&payloads); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid payload format"})
	}
	payloads.BatchID = uuid.New().String()
	data, err := json.Marshal(payloads)

	if err != nil {
		log.Printf("failed to marshal payload %v", err)
	}
	ctx := context.Background()

	redisClient.RPush(ctx, "qr_queue", data)

	return c.JSON(http.StatusAccepted, Response{Message: "QR generation queued"})
}

func getQRResult(c echo.Context) error {
	// --- START DIAGNOSTIC LOGGING ---
	log.Printf("[getQRResult] Request Path: %s", c.Path())
	log.Printf("[getQRResult] All Param Names: %v", c.ParamNames())
	log.Printf("[getQRResult] All Param Values: %v", c.ParamValues())
	rawIDParam := c.Param("id")
	log.Printf("[getQRResult] Raw c.Param(\"id\"): [%s]", rawIDParam)
	// --- END DIAGNOSTIC LOGGING ---

	ctx := context.Background()
	id := strings.TrimSpace(rawIDParam) // Use the rawIDParam we just logged
	log.Printf("[getQRResult] Trimmed id: [%s]", id)

	if id == "" {
		log.Printf("[getQRResult] ID is empty, returning error. Raw was: [%s]", rawIDParam)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "id is required"})
	}

	resultKey := "qr_result:" + id
	outputKey := "qr_result_output:" + id

	outputType, err := redisClient.Get(ctx, outputKey).Result()
	if err == redis.Nil {
		log.Printf("[getQRResult] Output type not found for ID=%s (redis:nil).", id)
		return c.JSON(http.StatusNotFound, echo.Map{"error": "output type not found or QR still processing"})
	} else if err != nil {
		log.Printf("[getQRResult] Error fetching output type for ID=%s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to retrieve QR metadata"})
	}

	data, err := redisClient.Get(ctx, resultKey).Bytes()
	if err == redis.Nil {
		log.Printf("[getQRResult] QR result data not found for ID=%s (redis:nil), but output type '%s' was found. Inconsistent state.", id, outputType)
		return c.JSON(http.StatusNotFound, echo.Map{"error": "QR result data not found (inconsistent state)"})
	} else if err != nil {
		log.Printf("[getQRResult] Redis get error for result data ID=%s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to retrieve QR data"})
	}

	log.Printf("[getQRResult] Successfully fetched data for ID=%s, OutputType=%s", id, outputType)

	if Output(outputType) == OutputBase64 {
		return c.JSON(http.StatusOK, Base64Response{ImageBase64: string(data)})
	}

	c.Response().Header().Set(echo.HeaderContentType, "image/png")
	c.Response().Header().Set("Content-Disposition", "inline; filename=\""+id+".png\"")
	return c.Blob(http.StatusOK, "image/png", data)
}

func generateCustomQR(c echo.Context) error {
	var req QRRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Data) == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "'data' is required"})
	}

	if strings.TrimSpace(req.ID) == "" {
		req.ID = uuid.New().String()
	}

	if req.Output != OutputBase64 {
		req.Output = OutputPNG
	}

	payload, _ := json.Marshal(req)
	if err := redisClient.RPush(context.Background(), "qr_queue", payload).Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to queue job"})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"message": "queued",
		"id":      req.ID,
	})
}

func generateBase64FromImage(c echo.Context) error {
	file, err := c.FormFile("image")
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Message: err.Error()})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Message: err.Error()})
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Message: "invalid image"})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, Base64Response{ImageBase64: base64.StdEncoding.EncodeToString(buf.Bytes())})
}

func qrWorker() {
	ctx := context.Background()
	for {
		res, err := redisClient.BLPop(ctx, 2*time.Minute, "qr_queue").Result()
		if err != nil {
			log.Printf("BlPop error %w", err)
			continue
		}
		// convert value of res to byte to unmarshal
		data := []byte(res[1])
		var batch BatchPayload
		if err := json.Unmarshal(data, &batch); err == nil && len(batch.QRRequests) > 0 {
			for _, req := range batch.QRRequests {
				if req.ID == "" {
					req.ID = uuid.New().String()
				}
				b64, err, pngBytes := ProcessQR(req)
				if err != nil {
					log.Printf("Process qr Failed for id=%s %v", req.ID, err)
					continue
				}
				resultKey := "qr_result:" + req.ID
				outputKey := "qr_result_output:" + req.ID
				if req.Output == OutputBase64 {
					if err := redisClient.Set(ctx, resultKey, b64, 5*time.Hour).Err(); err != nil {
						log.Printf("failed to store base64 for ID=%s %v", req.ID, err)
					}
				} else {
					if err := redisClient.Set(ctx, resultKey, pngBytes, 5*time.Hour).Err(); err != nil {
						log.Printf("failed to store png for ID=%s  %v", req.ID, err)
					}
				}
				if err := redisClient.Set(ctx, outputKey, string(req.Output), 5*time.Hour).Err(); err != nil {
					log.Printf("failed to store output type for ID=%s %v", req.ID, err)
				}
				log.Printf("Stored result for single ID=%s", req.ID)

			}
		}
	}
}

func ProcessQR(req QRRequest) (string, error, []byte) {
	if req.Size <= 0 {
		req.Size = 256
	}
	if req.FGColor == "" {
		req.FGColor = "#000000"
	}
	if req.BGColor == "" {
		req.BGColor = "#ffffff"
	}

	fg, err := hexToRGBA(req.FGColor)
	if err != nil {
		return "", err, nil
	}
	bg, err := hexToRGBA(req.BGColor)
	if err != nil {
		return "", err, nil
	}

	qr, err := qrcode.New(req.Data, qrcode.High)
	if err != nil {
		return "", err, nil
	}
	qr.ForegroundColor = fg
	qr.BackgroundColor = bg
	img := qr.Image(req.Size)

	dc := gg.NewContextForImage(img)
	if req.LogoBase64 != "" {
		if logo, err := decodeBase64Image(req.LogoBase64); err == nil {
			sz := req.Size / 4
			dc.DrawImageAnchored(resizeImage(logo, sz, sz), req.Size/2, req.Size/2, 0.5, 0.5)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return "", err, nil
	}

	if req.Output == OutputBase64 {
		return base64.StdEncoding.EncodeToString(buf.Bytes()), nil, nil
	}
	return "", nil, buf.Bytes()
}

func hexToRGBA(h string) (color.RGBA, error) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return color.RGBA{}, echo.NewHTTPError(http.StatusBadRequest, "invalid hex color")
	}
	r, err := strconv.ParseUint(h[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	g, err := strconv.ParseUint(h[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	b, err := strconv.ParseUint(h[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{uint8(r), uint8(g), uint8(b), 255}, nil
}

func decodeBase64Image(s string) (image.Image, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func resizeImage(img image.Image, w, h int) image.Image {
	dc := gg.NewContext(w, h)
	dc.DrawImageAnchored(img, w/2, h/2, 0.5, 0.5)
	return dc.Image()
}
