/*
Copyright 2022 The Magma Authors.

This source code is licensed under the BSD-style license found in the
LICENSE file in the root directory of this source tree.

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package servicers

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"magma/lte/cloud/go/lte"
	"magma/lte/cloud/go/serdes"
	"magma/lte/cloud/go/services/lte/obsidian/models"
	"magma/orc8r/cloud/go/services/configurator"
)

const ConfigCacheTTL = time.Minute * 10

// EpsAuthConfig stores all the configs needed to run the service.
type EpsAuthConfig struct {
	LteAuthOp   []byte
	LteAuthAmf  []byte
	SubProfiles map[string]models.NetworkEpcConfigsSubProfilesAnon
	lastUpdate  time.Time
}

type epsCfgCache struct {
	sync.RWMutex
	configs map[string]*EpsAuthConfig
}

var cfgCache = epsCfgCache{configs: map[string]*EpsAuthConfig{}}

// getConfig returns the EpsAuthConfig config for a given networkId.
func getConfig(networkID string) (*EpsAuthConfig, error) {
	now := time.Now()
	cfgCache.RLock()
	cfg, found := cfgCache.configs[networkID]
	cfgCache.RUnlock()

	if found && cfg != nil && now.Before(cfg.lastUpdate.Add(ConfigCacheTTL)) {
		return cfg, nil
	}

	iCellularConfigs, err := configurator.LoadNetworkConfig(
		context.Background(), networkID, lte.CellularNetworkConfigType, serdes.Network)
	if err != nil {
		return nil, err
	}
	if iCellularConfigs == nil {
		return nil, status.Error(codes.NotFound, "got nil when looking up config")
	}
	cellularConfig, ok := iCellularConfigs.(*models.NetworkCellularConfigs)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, "failed to convert config")
	}
	epc := cellularConfig.Epc
	cfg = &EpsAuthConfig{
		LteAuthOp:   epc.LteAuthOp,
		LteAuthAmf:  epc.LteAuthAmf,
		SubProfiles: epc.SubProfiles,
		lastUpdate:  now,
	}

	cfgCache.Lock()
	cfgCache.configs[networkID] = cfg
	cfgCache.Unlock()

	return cfg, nil
}
