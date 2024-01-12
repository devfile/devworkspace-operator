// Copyright (c) 2019-2024 Red Hat, Inc.
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

package kubernetes

import (
	"fmt"
	gosync "sync"

	"github.com/devfile/devworkspace-operator/pkg/provision/sync"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	decoder      runtime.Decoder
	decoderMutex gosync.Mutex
)

func InitializeDeserializer(scheme *runtime.Scheme) error {
	decoderMutex.Lock()
	defer decoderMutex.Unlock()
	if decoder != nil {
		return fmt.Errorf("attempting to re-initialize kubernetes object decoder")
	}
	decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
	return nil
}

func deserializeToObject(jsonObj []byte, api sync.ClusterAPI) (client.Object, error) {
	if decoder == nil {
		return nil, fmt.Errorf("kubernetes object deserializer is not initialized")
	}
	obj, _, err := decoder.Decode(jsonObj, nil, nil)
	if err != nil {
		return nil, err
	}
	if obj.GetObjectKind().GroupVersionKind().Kind == "List" {
		return nil, fmt.Errorf("objects of kind 'List' are unsupported")
	}
	clientObj, ok := obj.(client.Object)
	if !ok {
		// Should never occur but to avoid a panic
		return nil, fmt.Errorf("object does not have standard metadata and cannot be processed")
	}
	return clientObj, nil
}
