//
// Copyright (c) 2019-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

// Package config is used by components to get configuration.
//
// Typically each configuration property has the default value.
// Default value is supposed to be overridden via config map.
//
// There is the following configuration names convention:
// - words are lower-cased
// - . is used to separate subcomponents
// - _ is used to separate words in the component name
//
package config
