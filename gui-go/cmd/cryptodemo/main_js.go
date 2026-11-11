//go:build js && wasm

// Command cryptodemo exposes the public Pulse-Vault stream API to the local
// documentation demo. The browser runs this WASM module in a dedicated worker.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"syscall/js"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

const samplePlaintext = "Research starts with a question. An authenticated stream keeps these sample bytes private until each record passes its integrity check."

var (
	messageHandler js.Func
	demoKey        []byte
	demoCiphertext []byte
)

func main() {
	messageHandler = js.FuncOf(handleMessage)
	js.Global().Set("onmessage", messageHandler)
	postMessage(map[string]any{"type": "ready"})
	select {}
}

func handleMessage(_ js.Value, args []js.Value) any {
	if len(args) == 0 {
		return nil
	}
	event := args[0]
	data := event.Get("data")
	id := data.Get("id").Int()
	op := data.Get("op").String()
	go runDemoOperation(id, op)
	return nil
}

func runDemoOperation(id int, op string) {
	switch op {
	case "encrypt":
		encryptSample(id)
	case "decrypt":
		decryptSample(id)
	case "reset":
		demoKey = nil
		demoCiphertext = nil
		postMessage(map[string]any{"id": id, "type": "reset"})
	default:
		postMessage(map[string]any{"id": id, "type": "error", "message": "Unknown demo operation."})
	}
}

func encryptSample(id int) {
	profile := pulsecrypto.Profiles["standard"]
	salt := make([]byte, pulsecrypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		postError(id, err)
		return
	}
	postMessage(map[string]any{"id": id, "type": "progress", "stage": "kdf"})
	key, err := pulsecrypto.DeriveKeyScrypt("public-demo-passphrase", salt, profile.N, profile.R, profile.P)
	if err != nil {
		postError(id, err)
		return
	}
	postMessage(map[string]any{"id": id, "type": "progress", "stage": "stream"})
	var encrypted bytes.Buffer
	if err := pulsecrypto.EncryptStreamV6(key, bytes.NewReader([]byte(samplePlaintext)), &encrypted, true); err != nil {
		postError(id, err)
		return
	}
	demoKey = key
	demoCiphertext = append(demoCiphertext[:0], encrypted.Bytes()...)
	postMessage(map[string]any{
		"id":             id,
		"type":           "encrypted",
		"format":         pulsecrypto.StreamV6Magic,
		"salt":           base64.RawStdEncoding.EncodeToString(salt),
		"ciphertext":     base64.RawStdEncoding.EncodeToString(demoCiphertext),
		"cipherBytes":    len(demoCiphertext),
		"plaintextBytes": len(samplePlaintext),
	})
}

func decryptSample(id int) {
	if len(demoKey) == 0 || len(demoCiphertext) == 0 {
		postMessage(map[string]any{"id": id, "type": "error", "message": "Encrypt the sample first."})
		return
	}
	var plaintext bytes.Buffer
	if err := pulsecrypto.DecryptStreamV6(demoKey, bytes.NewReader(demoCiphertext), &plaintext); err != nil {
		postError(id, err)
		return
	}
	postMessage(map[string]any{
		"id":        id,
		"type":      "decrypted",
		"plaintext": plaintext.String(),
	})
}

func postError(id int, err error) {
	postMessage(map[string]any{"id": id, "type": "error", "message": fmt.Sprintf("Demo failed: %v", err)})
}

func postMessage(value map[string]any) {
	js.Global().Call("postMessage", js.ValueOf(value))
}
