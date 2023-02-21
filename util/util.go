package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rabobank/mfsb/conf"
	"github.com/rabobank/mfsb/model"
	"io"
	mathrand "math/rand"
	"net/http"
	"sync/atomic"
)

var lastUsedAZIndex int32 = 99

func WriteHttpResponse(w http.ResponseWriter, code int, object interface{}) {
	data, err := json.Marshal(object)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, err.Error())
		return
	}

	w.WriteHeader(code)
	_, _ = fmt.Fprintf(w, string(data))
	//fmt.Printf("response: code:%d, body: %s\n", code, string(data))  // credential leak!
}

// BasicAuth - validate if user/pass in the http request match the configured service broker user/pass
func BasicAuth(w http.ResponseWriter, r *http.Request, username, password string) bool {
	user, pass, ok := r.BasicAuth()
	if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="`+conf.BasicAuthRealm+`"`)
		w.WriteHeader(401)
		_, _ = w.Write([]byte("Unauthorised.\n"))
		return false
	}
	return true
}

func DumpRequest(r *http.Request) {
	if conf.Debug {
		fmt.Printf("dumping %s request for URL: %s\n", r.Method, r.URL)
		fmt.Println("dumping request headers...")
		// Loop over header names
		for name, values := range r.Header {
			if name == "Authorization" {
				fmt.Printf(" %s: %s\n", name, "<redacted>")
			} else {
				// Loop over all values for the name.
				for _, value := range values {
					fmt.Printf(" %s: %s\n", name, value)
				}
			}
		}

		// dump the request body
		fmt.Println("dumping request body...")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("Error reading body: %v\n", err)
		} else {
			fmt.Println(string(body))
		}
		// Restore the io.ReadCloser to it's original state
		r.Body = io.NopCloser(bytes.NewBuffer(body))
	}
}

func ProvisionObjectFromRequest(r *http.Request, object interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("failed to read json object from request, error: %s\n", err)
		return err
	}
	fmt.Printf("received body:%v\n", string(body))
	err = json.Unmarshal(body, object)
	if err != nil {
		fmt.Printf("failed to parse json object from request, error: %s\n", err)
		return err
	}
	return nil
}

func SafeSubstring(name string, maxLen int) string {
	if len(name) < maxLen {
		return name
	}
	return name[0:maxLen]
}

func GenerateGUID() string {
	ba := make([]byte, 16)
	_, err := rand.Read(ba)
	if err != nil {
		fmt.Printf("failed to generate guid, err: %s\n", err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", ba[0:4], ba[4:6], ba[6:8], ba[8:10], ba[10:])
}

func GetPlan(serviceId, planId string) model.ServicePlan {
	var planNotFound model.ServicePlan
	for _, service := range conf.Catalog.Services {
		if service.Id == serviceId {
			for _, plan := range service.Plans {
				if plan.Id == planId {
					return plan
				}
			}
		}
	}
	return planNotFound
}

func GetServiceById(serviceId string) model.Service {
	var service model.Service
	for _, service := range conf.Catalog.Services {
		if service.Id == serviceId {
			return service
		}
	}
	return service
}

func Encrypt(stringToEncrypt string) (string, error) {
	var encryptedString string
	if len(stringToEncrypt) == 0 {
		return "", nil
	}
	key, _ := hex.DecodeString(hex.EncodeToString([]byte(conf.EncryptKey)))

	//Since the key is in string, we need to convert decode it to bytes
	plaintext := []byte(stringToEncrypt)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return encryptedString, err
	}

	//Create a new GCM - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	//https://golang.org/pkg/crypto/cipher/#NewGCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return encryptedString, err
	}

	//Create a nonce. Nonce should be from GCM
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return encryptedString, err
	}

	//Encrypt the data using aesGCM.Seal
	//Since we don't want to save the nonce somewhere else in this case, we add it as a prefix to the encrypted data. The first nonce argument in Seal is the prefix.
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	encryptedString = fmt.Sprintf("%x", ciphertext)
	return encryptedString, nil
}

func Decrypt(encryptedString string) (string, error) {
	var decryptedString string
	if len(encryptedString) == 0 {
		return "", nil
	}
	key, _ := hex.DecodeString(hex.EncodeToString([]byte(conf.EncryptKey)))
	enc, _ := hex.DecodeString(encryptedString)

	//Create a new Cipher Block from the key
	block, err := aes.NewCipher(key)
	if err != nil {
		return decryptedString, err
	}

	//Create a new GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return decryptedString, err
	}

	//Get the nonce size
	nonceSize := aesGCM.NonceSize()

	//Extract the nonce from the encrypted data
	if nonceSize > len(enc) {
		return "", errors.New(fmt.Sprintf("invalid encrypted string, size (%d) is smaller than the nonce size (%d)", nonceSize, len(enc)))
	}
	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	//Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return decryptedString, err
	}

	decryptedString = fmt.Sprintf("%s", plaintext)
	return decryptedString, nil
}

// GetNextAZ this will return the "next" AZ, in order to evenly spread the instances
func GetNextAZ() *string {
	// get a random starting point between 0,1,2
	if lastUsedAZIndex == 99 {
		lastUsedAZIndex = int32(mathrand.Intn(len(conf.AZS)))
	}
	atomic.AddInt32(&lastUsedAZIndex, 1)
	if lastUsedAZIndex >= int32(len(conf.AZS)) {
		atomic.StoreInt32(&lastUsedAZIndex, 0)
	}
	return conf.AZS[lastUsedAZIndex]
}
