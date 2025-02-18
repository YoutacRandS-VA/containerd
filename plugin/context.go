/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package plugin

import (
	"context"
	"fmt"

	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
)

// InitContext is used for plugin initialization
type InitContext struct {
	Context           context.Context
	Properties        map[string]string
	Config            interface{}
	RegisterReadiness func() func()

	// Meta is metadata plugins can fill in at init
	Meta *Meta

	plugins *Set
}

// NewContext returns a new plugin InitContext
func NewContext(ctx context.Context, plugins *Set, properties map[string]string) *InitContext {
	if properties == nil {
		properties = map[string]string{}
	}
	return &InitContext{
		Context:    ctx,
		Properties: properties,
		Meta: &Meta{
			Exports: map[string]string{},
		},
		plugins: plugins,
	}
}

// Get returns the first plugin by its type
func (i *InitContext) Get(t Type) (interface{}, error) {
	return i.plugins.Get(t)
}

// Meta contains information gathered from the registration and initialization
// process.
type Meta struct {
	Platforms    []imagespec.Platform // platforms supported by plugin
	Exports      map[string]string    // values exported by plugin
	Capabilities []string             // feature switches for plugin
}

// Plugin represents an initialized plugin, used with an init context.
type Plugin struct {
	Registration Registration // registration, as initialized
	Config       interface{}  // config, as initialized
	Meta         Meta

	instance interface{}
	err      error // will be set if there was an error initializing the plugin
}

// Err returns the errors during initialization.
// returns nil if no error was encountered
func (p *Plugin) Err() error {
	return p.err
}

// Instance returns the instance and any initialization error of the plugin
func (p *Plugin) Instance() (interface{}, error) {
	return p.instance, p.err
}

// Set defines a plugin collection, used with InitContext.
//
// This maintains ordering and unique indexing over the set.
//
// After iteratively instantiating plugins, this set should represent, the
// ordered, initialization set of plugins for a containerd instance.
type Set struct {
	ordered     []*Plugin // order of initialization
	byTypeAndID map[Type]map[string]*Plugin
}

// NewPluginSet returns an initialized plugin set
func NewPluginSet() *Set {
	return &Set{
		byTypeAndID: make(map[Type]map[string]*Plugin),
	}
}

// Add a plugin to the set
func (ps *Set) Add(p *Plugin) error {
	if byID, typeok := ps.byTypeAndID[p.Registration.Type]; !typeok {
		ps.byTypeAndID[p.Registration.Type] = map[string]*Plugin{
			p.Registration.ID: p,
		}
	} else if _, idok := byID[p.Registration.ID]; !idok {
		byID[p.Registration.ID] = p
	} else {
		return fmt.Errorf("plugin add failed for %s: %w", p.Registration.URI(), ErrPluginInitialized)
	}

	ps.ordered = append(ps.ordered, p)
	return nil
}

// Get returns the first plugin by its type
func (ps *Set) Get(t Type) (interface{}, error) {
	for _, v := range ps.byTypeAndID[t] {
		return v.Instance()
	}
	return nil, fmt.Errorf("no plugins registered for %s: %w", t, ErrPluginNotFound)
}

// GetByID returns the plugin of the given type and ID
func (ps *Set) GetByID(t Type, id string) (*Plugin, error) {
	typSet, ok := ps.byTypeAndID[t]
	if !ok || len(typSet) == 0 {
		return nil, fmt.Errorf("no plugins registered for %s: %w", t, ErrPluginNotFound)
	}
	p, ok := typSet[id]
	if !ok {
		return nil, fmt.Errorf("no plugins registered for %s %q: %w", t, id, ErrPluginNotFound)
	}
	return p, nil
}

// GetAll returns all initialized plugins
func (ps *Set) GetAll() []*Plugin {
	return ps.ordered
}

// Plugins returns plugin set
func (i *InitContext) Plugins() *Set {
	return i.plugins
}

// GetAll plugins in the set
func (i *InitContext) GetAll() []*Plugin {
	return i.plugins.GetAll()
}

// GetByID returns the plugin of the given type and ID
func (i *InitContext) GetByID(t Type, id string) (interface{}, error) {
	ps, err := i.GetByType(t)
	if err != nil {
		return nil, err
	}
	p, ok := ps[id]
	if !ok {
		return nil, fmt.Errorf("no %s plugins with id %s: %w", t, id, ErrPluginNotFound)
	}
	return p.Instance()
}

// GetByType returns all plugins with the specific type.
func (i *InitContext) GetByType(t Type) (map[string]*Plugin, error) {
	p, ok := i.plugins.byTypeAndID[t]
	if !ok {
		return nil, fmt.Errorf("no plugins registered for %s: %w", t, ErrPluginNotFound)
	}

	return p, nil
}
