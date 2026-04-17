// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	_ "embed"
	"fmt"

	"github.com/rs/zerolog"
	"go.mondoo.com/mql/v13"
	"go.mondoo.com/mql/v13/exec"
	"go.mondoo.com/mql/v13/llx"
	"go.mondoo.com/mql/v13/mqlc"
	"go.mondoo.com/mql/v13/providers"
	"go.mondoo.com/mql/v13/providers-sdk/v1/inventory"
	pp "go.mondoo.com/mql/v13/providers-sdk/v1/plugin"
	coreconf "go.mondoo.com/mql/v13/providers/core/config"
	osconf "go.mondoo.com/mql/v13/providers/os/config"
	"go.mondoo.com/mql/v13/providers/os/connection/shared"
	osprovider "go.mondoo.com/mql/v13/providers/os/provider"
)

//go:embed schemas/os.resources.json
var osSchemaData []byte

//go:embed schemas/core.resources.json
var coreSchemaData []byte

// Engine wraps the MQL runtime for executing queries against the local system.
type Engine struct {
	runtime *providers.Runtime
}

// New creates a new Engine with the OS and core providers compiled in.
func New() (*Engine, error) {
	// Suppress MQL engine debug/trace/warn logging
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	osSchema := providers.MustLoadSchema("os", osSchemaData)
	coreSchema := providers.MustLoadSchema("core", coreSchemaData)
	mergedSchema := osSchema.Add(coreSchema)

	schema := providers.Coordinator.Schema().(providers.ExtensibleSchema)
	schema.Add(coreconf.Config.ID, coreSchema)
	schema.Add(osconf.Config.Name, osSchema)

	runtime := providers.Coordinator.NewRuntime()
	providers.Coordinator.DeactivateProviderDiscovery()

	provider := &providers.RunningProvider{
		Name:   osconf.Config.Name,
		ID:     osconf.Config.ID,
		Plugin: osprovider.Init(),
		Schema: mergedSchema,
	}
	runtime.Provider = &providers.ConnectedProvider{Instance: provider}
	runtime.AddConnectedProvider(runtime.Provider)

	runtime.AutoUpdate = providers.UpdateProvidersConfig{
		Enabled: false,
	}

	// Connect the OS provider to the local system
	connectedProvider := runtime.Provider
	connectRes, err := provider.Plugin.Connect(&pp.ConnectReq{
		Asset: &inventory.Asset{
			Connections: []*inventory.Config{{
				Type: shared.Type_Local.String(),
			}},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect OS provider: %w", err)
	}
	connectedProvider.Connection = connectRes

	return &Engine{runtime: runtime}, nil
}

// Close shuts down the engine runtime.
func (e *Engine) Close() {
	if e.runtime != nil {
		e.runtime.Close()
	}
}

// ExecSingle runs a single-value MQL query (e.g., "claude.code.skills").
func (e *Engine) ExecSingle(query string) (*llx.RawData, error) {
	result, err := exec.Exec(query, e.runtime, mql.Features{}, mqlc.EmptyPropsHandler)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", query, err)
	}
	return result, nil
}
