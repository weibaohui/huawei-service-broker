package rest

import (
	"net/http"
)

func DoHTTPrequest(url, endpoint, path string, headers, params map[string]string, body []byte) (http.Response, error) {

	return http.Response{}, nil
}
func CopyResponseBody(response http.Response) ([]byte, error) {
	return []byte{}, nil
}
func IsResponseStatusOk(response http.Response) bool {
	return true
}
func CloseResponseBody(response http.Response) {

}
