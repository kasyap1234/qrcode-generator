package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	"github.com/labstack/echo/v4"
	"github.com/skip2/go-qrcode"
)

type QRRequest struct {
	Data       string `json:"data"`                  // required
	Size       int    `json:"size"`                  // optional (default 256)
	FGColor    string `json:"fg_color"`              // optional (default #000000)
	BGColor    string `json:"bg_color"`              // optional (default #ffffff)
	LogoBase64 string `json:"logo_base64,omitempty"` // optional
}
type Response struct {
	Message string
}
type Image struct {
}
type Base64Response struct {
	ImageBase64 string `json:"image_base64"`
}

func main() {
	e := echo.New()
	e.POST("/generate", generateCustomQR)
	e.POST("/generateBase64", generateBase64FromImage)
	log.Fatal(e.Start(":8080"))
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
	// decoding to verify image
	img, _, err := image.Decode(src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Message: err.Error()})
	}
	defer src.Close()
	var buf bytes.Buffer
	// encoding it again
	if err := png.Encode(&buf, img); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Message: err.Error()})
	}
	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
	return c.JSON(http.StatusOK, Response{Message: base64Str})
}

type BatchRequest []QRRequest

func GenerateBatchQR(c echo.Context) error {
	var req BatchRequest
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, Response{Message: err.Error()})
	}

}

// func generateCustomQRBatch(c echo.Context) error {
// 	var req []QRRequest
// 	if err := c.Bind(req); err != nil {
// 		return c.JSON(http.StatusBadRequest, Response{Message: err.Error()})
// 	}
// 	return c.JSON(http.StatusOK, Response{Message: "success"})
// }

func generateCustomQR(c echo.Context) error {
	var req QRRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil || strings.TrimSpace(req.Data) == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid request. 'data' is required."})
	}

	if req.Size <= 0 {
		req.Size = 256
	}
	if req.FGColor == "" {
		req.FGColor = "#000000"
	}
	if req.BGColor == "" {
		req.BGColor = "#ffffff"
	}

	// Parse hex colors
	fg, err := hexToRGBA(req.FGColor)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid fg_color"})
	}
	bg, err := hexToRGBA(req.BGColor)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid bg_color"})
	}

	// Generate QR
	qr, err := qrcode.New(req.Data, qrcode.High)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to create QR code"})
	}
	qr.ForegroundColor = fg
	qr.BackgroundColor = bg

	qrImg := qr.Image(req.Size)

	// Add logo if provided
	dc := gg.NewContextForImage(qrImg)
	if req.LogoBase64 != "" {
		logoImg, err := decodeBase64Image(req.LogoBase64)
		if err == nil {
			logoSize := req.Size / 4
			logoResized := resizeImage(logoImg, logoSize, logoSize)
			dc.DrawImageAnchored(logoResized, req.Size/2, req.Size/2, 0.5, 0.5)
		}
	}

	// Output as PNG or Base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to encode image"})
	}

	outputType := c.QueryParam("output")
	if outputType == "base64" {
		encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
		return c.JSON(http.StatusOK, Base64Response{ImageBase64: encoded})
	}

	return c.Blob(http.StatusOK, "image/png", buf.Bytes())
}

// hexToRGBA converts a hex string like "#ff00ff" to color.RGBA
func hexToRGBA(hex string) (color.RGBA, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{}, echo.NewHTTPError(http.StatusBadRequest, "Invalid hex color")
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
}

// decodeBase64Image parses a base64 string into an image.Image
func decodeBase64Image(base64Str string) (image.Image, error) {
	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// resizeImage resizes an image using gg context
func resizeImage(img image.Image, width, height int) image.Image {
	dc := gg.NewContext(width, height)
	dc.DrawImageAnchored(img, width/2, height/2, 0.5, 0.5)
	return dc.Image()
}
