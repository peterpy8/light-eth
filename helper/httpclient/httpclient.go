package httpclient

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/siotchain/siot/helper"
	"github.com/siotchain/siot/crypto"
)

type HTTPClient struct {
	*http.Transport
	DocRoot string
	schemes []string
}

func New(docRoot string) (self *HTTPClient) {
	self = &HTTPClient{
		Transport: &http.Transport{},
		DocRoot:   docRoot,
		schemes:   []string{"file"},
	}
	self.RegisterProtocol("file", http.NewFileTransport(http.Dir(self.DocRoot)))
	return
}

// Clients should be reused instead of created as needed. Clients are safe for concurrent use by multiple goroutines.

// A Client is higher-level than a RoundTripper (such as Transport) and additionally handles HTTP details such as cookies and redirects.

func (self *HTTPClient) Client() *http.Client {
	return &http.Client{
		Transport: self,
	}
}

func (self *HTTPClient) RegisterScheme(scheme string, rt http.RoundTripper) {
	self.schemes = append(self.schemes, scheme)
	self.RegisterProtocol(scheme, rt)
}

func (self *HTTPClient) HasScheme(scheme string) bool {
	for _, s := range self.schemes {
		if s == scheme {
			return true
		}
	}
	return false
}

func (self *HTTPClient) GetAuthContent(uri string, hash helper.Hash) ([]byte, error) {
	// retrieve content
	content, err := self.Get(uri, "")
	if err != nil {
		return nil, err
	}

	// check hash to authenticate content
	chash := crypto.Keccak256Hash(content)
	if chash != hash {
		return nil, fmt.Errorf("content hash mismatch %x != %x (exp)", hash[:], chash[:])
	}

	return content, nil

}

// Get(uri, path) downloads the document at uri, if path is non-empty it
// is interpreted as a filepath to which the contents are saved
func (self *HTTPClient) Get(uri, path string) ([]byte, error) {
	// retrieve content
	resp, err := self.Client().Get(uri)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	var content []byte
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 != 2 {
		return content, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	if path != "" {
		var abspath string
		abspath, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		err = ioutil.WriteFile(abspath, content, 0600)
		if err != nil {
			return nil, err
		}
	}

	return content, nil

}
