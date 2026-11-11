package runninghub

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// putCOSObject uploads bytes to Tencent COS via S3-compatible SigV4 PutObject
// (same shape as Runow aws-sdk-js PutObject with x-amz-acl=public-read).
func putCOSObject(httpClient *http.Client, cred QCStorageCreds, objectKey string, body []byte, contentType string) error {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	objectKey = strings.TrimLeft(objectKey, "/")
	if objectKey == "" {
		return fmt.Errorf("cos object key required")
	}
	if strings.TrimSpace(cred.AccessKey) == "" || strings.TrimSpace(cred.SecretKey) == "" {
		return fmt.Errorf("cos credentials incomplete")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	host := cred.Bucket + ".cos." + cred.Region + ".myqcloud.com"
	payloadHash := sha256Hex(body)
	amzDate := time.Now().UTC().Format("20060102T150405Z")
	dateStamp := amzDate[:8]
	token := strings.TrimSpace(cred.SessionToken)

	query := url.Values{}
	query.Set("x-id", "PutObject")
	canonicalQuery := query.Encode()

	headers := map[string]string{
		"content-type":         contentType,
		"host":                 host,
		"x-amz-acl":            "public-read",
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if token != "" {
		headers["x-amz-security-token"] = token
	}

	signedNames := make([]string, 0, len(headers))
	for k := range headers {
		signedNames = append(signedNames, k)
	}
	sort.Strings(signedNames)
	var canonicalHeader strings.Builder
	for _, k := range signedNames {
		canonicalHeader.WriteString(k)
		canonicalHeader.WriteByte(':')
		canonicalHeader.WriteString(strings.TrimSpace(headers[k]))
		canonicalHeader.WriteByte('\n')
	}
	signedHeaders := strings.Join(signedNames, ";")

	canonicalURI := "/" + objectKey
	canonicalRequest := strings.Join([]string{
		"PUT",
		canonicalURI,
		canonicalQuery,
		canonicalHeader.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := dateStamp + "/" + cred.Region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := aws4SigningKey(cred.SecretKey, dateStamp, cred.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	authorization := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		cred.AccessKey, credentialScope, signedHeaders, signature,
	)

	u := "https://" + host + canonicalURI + "?" + canonicalQuery
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	for k, v := range headers {
		if k == "host" {
			continue
		}
		req.Header.Set(k, v)
	}
	req.ContentLength = int64(len(body))

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("cos put HTTP %d: %s", res.StatusCode, truncate(string(raw), 300))
	}
	return nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}

func aws4SigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}
