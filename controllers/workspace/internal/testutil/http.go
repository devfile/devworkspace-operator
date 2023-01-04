// Copyright (c) 2019-2023 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type TestRoundTripper struct {
	Data map[string]TestResponse
}

type TestResponse struct {
	StatusCode int
	Bytes      []byte
	Err        error
}

func (rt *TestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return nil, fmt.Errorf("test HTTP client only supports GET requests")
	}
	resp, ok := rt.Data[req.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected request URL in test HTTP client: %s", req.URL.String())
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	return &http.Response{
		StatusCode:    resp.StatusCode,
		Body:          io.NopCloser(bytes.NewBuffer(resp.Bytes)),
		ContentLength: int64(len(resp.Bytes)),
	}, nil
}

var _ http.RoundTripper = (*TestRoundTripper)(nil)
