package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

func makeSig(key []byte, payload string) ([]byte, error) {
	mac := hmac.New(sha256.New, key)
	_, err := mac.Write([]byte(payload))
	if err != nil {
		return nil, err
	}
	sig := mac.Sum(nil)[:20]
	return sig, nil
}

func generateAuth(key []byte) string {
	payload := fmt.Sprintf("%x", time.Now().Unix()+12*3600)
	sig, err := makeSig(key, payload)
	httpCheck(err)
	return base64.StdEncoding.EncodeToString(append(sig, []byte(payload)...))
}

func verifyAuth(key []byte, s string) {
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		abortUserError("Bad auth cookie.")
	}
	sig := buf[:20]
	payload := string(buf[20:])
	xsig, err := makeSig(key, payload)
	if err != nil || !hmac.Equal(sig, xsig) {
		abortUserError("Bad auth cookie..")
	}
	tm, err := strconv.ParseInt(payload, 16, 32)
	if err != nil {
		abortUserError("Bad auth cookie...")
	}
	if time.Now().Unix() > tm {
		abortUserError("Bad auth cookie....")
	}
}
