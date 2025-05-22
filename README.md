

## 🧾 QR Code Generator API

This is a Go-based backend service built with [Echo](https://echo.labstack.com/) that generates QR codes with optional custom IDs and retrieves batch QR code results.

---

### 📦 Features

* Generate QR codes with optional custom IDs.
* Retrieve a single QR code by ID.
* Fetch multiple QR code results using batch request.
* Fast and lightweight REST API.

---

### 🚀 Getting Started

#### ✅ Requirements

* Go 1.20+
* `go mod tidy` dependencies already installed

#### 🛠 Run the server

```bash
go run main.go
```

The server will start on `http://localhost:8080`.

---

### 📚 API Documentation

#### 1. **Generate QR Code**

**Endpoint**: `POST /generate`

**Request Body**:

```json
{
  "content": "https://example.com",
  "custom_id": "qr-1" // optional
}
```

* `content` (string, required): The content to encode into the QR code.
* `custom_id` (string, optional): If provided, assigns a custom ID to the QR. Otherwise, a random ID will be generated.

**Curl Example**:

```bash
curl -X POST http://localhost:8080/generate \
  -H "Content-Type: application/json" \
  -d '{
    "content": "https://example.com",
    "custom_id": "qr-1"
}'
```

**Response**:

```json
{
  "id": "qr-1",
  "qr_image": "data:image/png;base64,iVBORw0KGgo..."
}
```

---

#### 2. **Get QR Code by ID**

**Endpoint**: `GET /qr/:id`

**Example**:

```bash
curl http://localhost:8080/qr/qr-1
```

**Response**:

```json
{
  "id": "qr-1",
  "qr_image": "data:image/png;base64,..."
}
```

---

#### 3. **Batch QR Results**

**Endpoint**: `POST /batch-results`

**Request Body**:

```json
{
  "ids": ["qr-1", "qr-2"]
}
```

**Curl Example**:

```bash
curl -X POST http://localhost:8080/batch-results \
  -H "Content-Type: application/json" \
  -d '{
    "ids": ["qr-1", "qr-2"]
}'
```

**Response**:

```json
{
  "results": [
    {
      "id": "qr-1",
      "qr_image": "data:image/png;base64,..."
    },
    {
      "id": "qr-2",
      "qr_image": "data:image/png;base64,..."
    }
  ]
}
```

---



* Add persistent storage (e.g., Redis).
----
# To do
* Add expiration time for generated QR codes.
* Add Swagger/OpenAPI support for interactive docs.
* Add unit tests.

---
