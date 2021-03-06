// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stn

// StnRulesPrefix stn key/
const StnRulesPrefix = "vpp/config/v1/stn/rules/"

// KeyPrefix returns the prefix used in ETCD to store vpp STN config
func KeyPrefix() string {
	return StnRulesPrefix
}

// Key returns the prefix used in ETCD to store vpp STN config
// of a particular rule in selected vpp instance.
func Key(ruleName string) string {
	return StnRulesPrefix + ruleName
}
