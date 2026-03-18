package models

import (
	"encoding/base64"
	"fmt"

	"github.com/skip2/go-qrcode"
)

// generateQRCode generates a QR code PNG for the given URL and returns
// a base64-encoded string and a unique filename based on the recipient ID.
func generateQRCode(url, rid string) (string, string, error) {
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return "", "", err
	}
	b64 := base64.StdEncoding.EncodeToString(png)
	name := fmt.Sprintf("qr_%s.png", rid)
	return b64, name, nil
}
