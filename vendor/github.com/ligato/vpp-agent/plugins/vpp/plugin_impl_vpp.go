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

package vpp

import (
	"context"
	"os"
	"sync"

	govppapi "git.fd.io/govpp.git/api"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/logging/measure"
	"github.com/ligato/cn-infra/messaging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/govppmux"
	ifaceLinux "github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/aclplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/vpp/ipsecplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/ipsecplugin/ipsecidx"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/l2plugin/l2idx"
	"github.com/ligato/vpp-agent/plugins/vpp/l3plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/l4plugin"
	"github.com/ligato/vpp-agent/plugins/vpp/l4plugin/nsidx"
	"github.com/ligato/vpp-agent/plugins/vpp/model/acl"
	intf "github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/vpp/model/nat"
	"github.com/ligato/vpp-agent/plugins/vpp/rpc"
	"github.com/ligato/vpp-agent/plugins/vpp/srplugin"
	"github.com/ligato/vpp-agent/plugins/vpp/srplugin/vppcalls"
	"github.com/namsral/flag"
)

// vpp-plugin specific flags
var (
	// skip resync flag
	skipResyncFlag = flag.Bool("skip-vpp-resync", false, "Skip vpp-plugin resync")
)

var (
	// noopWriter (no operation writer) helps avoiding NIL pointer based segmentation fault.
	// It is used as default if some dependency was not injected.
	noopWriter = &datasync.CompositeKVProtoWriter{Adapters: []datasync.KeyProtoValWriter{}}

	// noopWatcher (no operation watcher) helps avoiding NIL pointer based segmentation fault.
	// It is used as default if some dependency was not injected.
	noopWatcher = &datasync.CompositeKVProtoWatcher{Adapters: []datasync.KeyValProtoWatcher{}}
)

// VPP resync strategy. Can be set in vpp-plugin.conf. If no strategy is set, the default behavior is defined by 'fullResync'
const (
	// fullResync calls the full resync for every default plugin
	fullResync = "full"
	// optimizeColdStart checks existence of the configured interface on the VPP (except local0). If there are any, the full
	// resync is executed, otherwise it's completely skipped.
	// Note: resync will be skipped also in case there is not configuration in VPP but exists in etcd
	optimizeColdStart = "optimize"
	// resync is skipped in any case
	skipResync = "skip"
)

// Plugin implements Plugin interface, therefore it can be loaded with other plugins.
type Plugin struct {
	Deps

	// Configurators
	aclConfigurator      *aclplugin.ACLConfigurator
	ifConfigurator       *ifplugin.InterfaceConfigurator
	bfdConfigurator      *ifplugin.BFDConfigurator
	natConfigurator      *ifplugin.NatConfigurator
	stnConfigurator      *ifplugin.StnConfigurator
	ipSecConfigurator    *ipsecplugin.IPSecConfigurator
	bdConfigurator       *l2plugin.BDConfigurator
	fibConfigurator      *l2plugin.FIBConfigurator
	xcConfigurator       *l2plugin.XConnectConfigurator
	arpConfigurator      *l3plugin.ArpConfigurator
	proxyArpConfigurator *l3plugin.ProxyArpConfigurator
	routeConfigurator    *l3plugin.RouteConfigurator
	appNsConfigurator    *l4plugin.AppNsConfigurator
	srv6Configurator     *srplugin.SRv6Configurator

	// State updaters
	ifStateUpdater *ifplugin.InterfaceStateUpdater
	bdStateUpdater *l2plugin.BridgeDomainStateUpdater

	// Shared indexes
	swIfIndexes    ifaceidx.SwIfIndexRW
	linuxIfIndexes ifaceLinux.LinuxIfIndex
	bdIndexes      l2idx.BDIndexRW
	errorIndexes   idxvpp.NameToIdxRW
	errorIdxSeq    uint32

	// Channels (watch, notification, ...) which should be closed
	ifStateChan       chan *intf.InterfaceNotification
	ifIdxWatchCh      chan ifaceidx.SwIfIdxDto
	ifVppNotifChan    chan govppapi.Message
	bdStateChan       chan *l2plugin.BridgeDomainStateNotification
	bdIdxWatchCh      chan l2idx.BdChangeDto
	bdVppNotifChan    chan l2plugin.BridgeDomainStateMessage
	linuxIfIdxWatchCh chan ifaceLinux.LinuxIfIndexDto
	errorChannel      chan ErrCtx

	// Publishers
	ifStateNotifications messaging.ProtoPublisher

	// Resync
	resyncConfigChan chan datasync.ResyncEvent
	resyncStatusChan chan datasync.ResyncEvent
	changeChan       chan datasync.ChangeEvent //TODO dedicated type abstracted from ETCD
	watchConfigReg   datasync.WatchRegistration
	watchStatusReg   datasync.WatchRegistration
	omittedPrefixes  []string // list of keys which won't be resynced

	// From config file
	ifMtu           uint32
	resyncStrategy  string
	enableStopwatch bool

	// Common
	statusCheckReg bool
	cancel         context.CancelFunc // cancel can be used to cancel all goroutines and their jobs inside of the plugin
	wg             sync.WaitGroup     // wait group that allows to wait until all goroutines of the plugin have finished
}

// Deps groups injected dependencies of plugin so that they do not mix with
// other plugin fieldsMtu.
type Deps struct {
	// inject all below
	local.PluginInfraDeps

	Publish           datasync.KeyProtoValWriter
	PublishStatistics datasync.KeyProtoValWriter
	Watch             datasync.KeyValProtoWatcher
	IfStatePub        datasync.KeyProtoValWriter
	GoVppmux          govppmux.API
	Linux             linuxpluginAPI
	GRPCSvc           rpc.GRPCService

	DataSyncs        map[string]datasync.KeyProtoValWriter
	WatchEventsMutex *sync.Mutex
}

// PluginConfig holds the vpp-plugin configuration.
type PluginConfig struct {
	Mtu              uint32   `json:"mtu"`
	Stopwatch        bool     `json:"stopwatch"`
	Strategy         string   `json:"strategy"`
	StatusPublishers []string `json:"status-publishers"`
}

type linuxpluginAPI interface {
	// GetLinuxIfIndexes gives access to mapping of logical names (used in ETCD configuration) to corresponding Linux
	// interface indexes. This mapping is especially helpful for plugins that need to watch for newly added or deleted
	// Linux interfaces.
	GetLinuxIfIndexes() ifaceLinux.LinuxIfIndex
}

// DisableResync can be used to disable resync for one or more key prefixes
func (plugin *Plugin) DisableResync(keyPrefix ...string) {
	plugin.Log.Infof("Keys with prefixes %v will be skipped", keyPrefix)
	plugin.omittedPrefixes = keyPrefix
}

// GetSwIfIndexes gives access to mapping of logical names (used in ETCD configuration) to sw_if_index.
// This mapping is helpful if other plugins need to configure VPP by the Binary API that uses sw_if_index input.
func (plugin *Plugin) GetSwIfIndexes() ifaceidx.SwIfIndex {
	return plugin.ifConfigurator.GetSwIfIndexes()
}

// GetDHCPIndices gives access to mapping of logical names (used in ETCD configuration) to dhcp_index.
// This mapping is helpful if other plugins need to know about the DHCP configuration set by VPP.
func (plugin *Plugin) GetDHCPIndices() ifaceidx.DhcpIndex {
	return plugin.ifConfigurator.GetDHCPIndexes()
}

// GetBfdSessionIndexes gives access to mapping of logical names (used in ETCD configuration) to bfd_session_indexes.
func (plugin *Plugin) GetBfdSessionIndexes() idxvpp.NameToIdx {
	return plugin.bfdConfigurator.GetBfdSessionIndexes()
}

// GetBfdAuthKeyIndexes gives access to mapping of logical names (used in ETCD configuration) to bfd_auth_keys.
func (plugin *Plugin) GetBfdAuthKeyIndexes() idxvpp.NameToIdx {
	return plugin.bfdConfigurator.GetBfdKeyIndexes()
}

// GetBfdEchoFunctionIndexes gives access to mapping of logical names (used in ETCD configuration) to bfd_echo_function
func (plugin *Plugin) GetBfdEchoFunctionIndexes() idxvpp.NameToIdx {
	return plugin.bfdConfigurator.GetBfdEchoFunctionIndexes()
}

// GetBDIndexes gives access to mapping of logical names (used in ETCD configuration) as bd_indexes.
func (plugin *Plugin) GetBDIndexes() l2idx.BDIndex {
	return plugin.bdIndexes
}

// GetFIBIndexes gives access to mapping of logical names (used in ETCD configuration) as fib_indexes.
func (plugin *Plugin) GetFIBIndexes() l2idx.FIBIndexRW {
	return plugin.fibConfigurator.GetFibIndexes()
}

// GetXConnectIndexes gives access to mapping of logical names (used in ETCD configuration) as xc_indexes.
func (plugin *Plugin) GetXConnectIndexes() l2idx.XcIndexRW {
	return plugin.xcConfigurator.GetXcIndexes()
}

// GetAppNsIndexes gives access to mapping of app-namespace logical names (used in ETCD configuration)
// to their respective indices as assigned by VPP.
func (plugin *Plugin) GetAppNsIndexes() nsidx.AppNsIndex {
	return plugin.appNsConfigurator.GetAppNsIndexes()
}

// DumpIPACL returns a list of all configured IP ACL entires
func (plugin *Plugin) DumpIPACL() (acls []*acl.AccessLists_Acl, err error) {
	return plugin.aclConfigurator.DumpIPACL()
}

// DumpMACIPACL returns a list of all configured MACIP ACL entires
func (plugin *Plugin) DumpMACIPACL() (acls []*acl.AccessLists_Acl, err error) {
	return plugin.aclConfigurator.DumpMACIPACL()
}

// DumpNat44Global returns the current NAT44 global config
func (plugin *Plugin) DumpNat44Global() (*nat.Nat44Global, error) {
	return plugin.natConfigurator.DumpNatGlobal()
}

// DumpNat44DNat returns the current NAT44 DNAT config
func (plugin *Plugin) DumpNat44DNat() (*nat.Nat44DNat, error) {
	return plugin.natConfigurator.DumpNatDNat()
}

// GetIPSecSAIndexes
func (plugin *Plugin) GetIPSecSAIndexes() idxvpp.NameToIdx {
	return plugin.ipSecConfigurator.GetSaIndexes()
}

// GetIPSecSPDIndexes
func (plugin *Plugin) GetIPSecSPDIndexes() ipsecidx.SPDIndex {
	return plugin.ipSecConfigurator.GetSpdIndexes()
}

// Init gets handlers for ETCD and Messaging and delegates them to ifConfigurator & ifStateUpdater.
func (plugin *Plugin) Init() error {
	plugin.Log.Debug("Initializing default plugins")
	flag.Parse()

	// Read config file and set all related fields
	plugin.fromConfigFile()
	// Fills nil dependencies with default values
	plugin.fixNilPointers()
	// Set interface state publisher
	plugin.ifStateNotifications = plugin.Deps.IfStatePub

	// All channels that are used inside of publishIfStateEvents or watchEvents must be created in advance!
	plugin.ifStateChan = make(chan *intf.InterfaceNotification, 100)
	plugin.bdStateChan = make(chan *l2plugin.BridgeDomainStateNotification, 100)
	plugin.resyncConfigChan = make(chan datasync.ResyncEvent)
	plugin.resyncStatusChan = make(chan datasync.ResyncEvent)
	plugin.changeChan = make(chan datasync.ChangeEvent)
	plugin.ifIdxWatchCh = make(chan ifaceidx.SwIfIdxDto, 100)
	plugin.bdIdxWatchCh = make(chan l2idx.BdChangeDto, 100)
	plugin.linuxIfIdxWatchCh = make(chan ifaceLinux.LinuxIfIndexDto, 100)
	plugin.errorChannel = make(chan ErrCtx, 100)

	// Create plugin context, save cancel function into the plugin handle.
	var ctx context.Context
	ctx, plugin.cancel = context.WithCancel(context.Background())

	// FIXME: Run the following go routines later than following init*() calls - just before Watch().

	// Run event handler go routines.
	go plugin.publishIfStateEvents(ctx)
	go plugin.publishBdStateEvents(ctx)
	go plugin.watchEvents(ctx)

	// Run error handler.
	go plugin.changePropagateError()

	var err error
	if err = plugin.initIF(ctx); err != nil {
		return err
	}
	if err = plugin.initIPSec(ctx); err != nil {
		return err
	}
	if err = plugin.initACL(ctx); err != nil {
		return err
	}
	if err = plugin.initL2(ctx); err != nil {
		return err
	}
	if err = plugin.initL3(ctx); err != nil {
		return err
	}
	if err = plugin.initL4(ctx); err != nil {
		return err
	}
	if err = plugin.initSR(ctx); err != nil {
		return err
	}

	if err = plugin.initErrorHandler(); err != nil {
		return err
	}

	if err = plugin.subscribeWatcher(); err != nil {
		return err
	}

	return nil
}

// AfterInit delegates the call to ifStateUpdater.
func (plugin *Plugin) AfterInit() error {
	plugin.Log.Debug("vpp plugins AfterInit begin")

	err := plugin.ifStateUpdater.AfterInit()
	if err != nil {
		return err
	}

	if plugin.StatusCheck != nil {
		// Register the plugin to status check. Periodical probe is not needed,
		// data change will be reported when changed
		plugin.StatusCheck.Register(plugin.PluginName, nil)
		// Notify that status check for default plugins was registered. It will
		// prevent status report errors in case resync is executed before AfterInit
		plugin.statusCheckReg = true
	}

	plugin.Log.Debug("vpp plugins AfterInit finished successfully")

	return nil
}

// Close cleans up the resources.
func (plugin *Plugin) Close() error {
	plugin.cancel()
	plugin.wg.Wait()

	_, err := safeclose.CloseAll(
		// Configurators
		plugin.aclConfigurator, plugin.ifConfigurator, plugin.bfdConfigurator, plugin.natConfigurator, plugin.stnConfigurator,
		plugin.ipSecConfigurator, plugin.bdConfigurator, plugin.fibConfigurator, plugin.xcConfigurator, plugin.arpConfigurator,
		plugin.proxyArpConfigurator, plugin.routeConfigurator, plugin.appNsConfigurator,
		// State updaters
		plugin.ifStateUpdater, plugin.bdStateUpdater,
		// Channels
		plugin.ifStateChan, plugin.ifVppNotifChan, plugin.ifIdxWatchCh, plugin.bdStateChan, plugin.bdVppNotifChan, plugin.bdIdxWatchCh, plugin.linuxIfIdxWatchCh, plugin.resyncStatusChan, plugin.resyncConfigChan,
		plugin.changeChan, plugin.errorChannel,
		// Registrations
		plugin.watchStatusReg, plugin.watchConfigReg,
	)

	return err
}

// Resolves resync strategy. Skip resync flag is also evaluated here and it has priority regardless
// the resync strategy parameter.
func (plugin *Plugin) resolveResyncStrategy(strategy string) string {
	// first check skip resync flag
	if *skipResyncFlag {
		return skipResync
		// else check if strategy is set in configfile
	} else if strategy == fullResync || strategy == optimizeColdStart {
		return strategy
	}
	plugin.Log.Infof("Resync strategy %v is not known, setting up the full resync", strategy)
	return fullResync
}

// fixNilPointers sets noopWriter & nooWatcher for nil dependencies.
func (plugin *Plugin) fixNilPointers() {
	if plugin.Deps.Publish == nil {
		plugin.Deps.Publish = noopWriter
		plugin.Log.Debug("setting default noop writer for Publish dependency")
	}
	if plugin.Deps.PublishStatistics == nil {
		plugin.Deps.PublishStatistics = noopWriter
		plugin.Log.Debug("setting default noop writer for PublishStatistics dependency")
	}
	if plugin.Deps.IfStatePub == nil {
		plugin.Deps.IfStatePub = noopWriter
		plugin.Log.Debug("setting default noop writer for IfStatePub dependency")
	}
	if plugin.Deps.Watch == nil {
		plugin.Deps.Watch = noopWatcher
		plugin.Log.Debug("setting default noop watcher for Watch dependency")
	}
}

func (plugin *Plugin) initIF(ctx context.Context) error {
	plugin.Log.Infof("Init interface plugin")

	// Get pointer to the map with Linux interface indices.
	if plugin.Linux != nil {
		plugin.linuxIfIndexes = plugin.Linux.GetLinuxIfIndexes()
	} else {
		plugin.linuxIfIndexes = nil
	}

	// Interface configurator
	plugin.ifVppNotifChan = make(chan govppapi.Message, 100)
	plugin.ifConfigurator = &ifplugin.InterfaceConfigurator{}
	if err := plugin.ifConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.Linux, plugin.ifVppNotifChan, plugin.ifMtu, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("ifConfigurator Initialized")

	// Get interface indexes
	plugin.swIfIndexes = plugin.ifConfigurator.GetSwIfIndexes()

	// Interface state updater
	plugin.ifStateUpdater = &ifplugin.InterfaceStateUpdater{}
	plugin.ifStateUpdater.Init(plugin.Log, plugin.GoVppmux, ctx, plugin.swIfIndexes, plugin.ifVppNotifChan, func(state *intf.InterfaceNotification) {
		select {
		case plugin.ifStateChan <- state:
			// OK
		default:
			plugin.Log.Debug("Unable to send to the ifStateNotifications channel - channel buffer full.")
		}
	})

	plugin.Log.Debug("ifStateUpdater Initialized")

	// BFD configurator
	plugin.bfdConfigurator = &ifplugin.BFDConfigurator{}
	if err := plugin.bfdConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("bfdConfigurator Initialized")

	// STN configurator
	plugin.stnConfigurator = &ifplugin.StnConfigurator{}
	if err := plugin.stnConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("stnConfigurator Initialized")

	// NAT configurator
	plugin.natConfigurator = &ifplugin.NatConfigurator{}
	if err := plugin.natConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("natConfigurator Initialized")

	return nil
}

func (plugin *Plugin) initIPSec(ctx context.Context) (err error) {
	plugin.Log.Infof("Init IPSec plugin")

	// IPSec configurator
	plugin.ipSecConfigurator = &ipsecplugin.IPSecConfigurator{}
	if err = plugin.ipSecConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}

	plugin.Log.Debug("ipSecConfigurator Initialized")
	return nil
}

func (plugin *Plugin) initACL(ctx context.Context) error {
	plugin.Log.Infof("Init ACL plugin")

	// ACL configurator
	plugin.aclConfigurator = &aclplugin.ACLConfigurator{}
	err := plugin.aclConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch)
	if err != nil {
		return err
	}
	plugin.Log.Debug("aclConfigurator Initialized")

	return nil
}

func (plugin *Plugin) initL2(ctx context.Context) error {
	plugin.Log.Infof("Init L2 plugin")

	// Bridge domain configurator
	plugin.bdVppNotifChan = make(chan l2plugin.BridgeDomainStateMessage, 100)
	plugin.bdConfigurator = &l2plugin.BDConfigurator{}
	err := plugin.bdConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.bdVppNotifChan, plugin.enableStopwatch)
	if err != nil {
		return err
	}
	plugin.Log.Debug("bdConfigurator Initialized")

	// Get bridge domain indexes
	plugin.bdIndexes = plugin.bdConfigurator.GetBdIndexes()

	// Bridge domain state updater
	plugin.bdStateUpdater = &l2plugin.BridgeDomainStateUpdater{}
	if err := plugin.bdStateUpdater.Init(plugin.Log, plugin.GoVppmux, ctx, plugin.bdIndexes, plugin.swIfIndexes, plugin.bdVppNotifChan,
		func(state *l2plugin.BridgeDomainStateNotification) {
			select {
			case plugin.bdStateChan <- state:
				// OK
			default:
				plugin.Log.Debug("Unable to send to the bdState channel: buffer is full.")
			}
		}); err != nil {
		return err
	}

	// L2 FIB configurator
	plugin.fibConfigurator = &l2plugin.FIBConfigurator{}
	err = plugin.fibConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.bdIndexes, plugin.enableStopwatch)
	if err != nil {
		return err
	}
	plugin.Log.Debug("fibConfigurator Initialized")

	// L2 cross connect
	plugin.xcConfigurator = &l2plugin.XConnectConfigurator{}
	if err := plugin.xcConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("xcConfigurator Initialized")

	return nil
}

func (plugin *Plugin) initL3(ctx context.Context) error {
	plugin.Log.Infof("Init L3 plugin")

	// ARP configurator
	plugin.arpConfigurator = &l3plugin.ArpConfigurator{}
	if err := plugin.arpConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("arpConfigurator Initialized")

	// Proxy ARP configurator
	plugin.proxyArpConfigurator = &l3plugin.ProxyArpConfigurator{}
	if err := plugin.proxyArpConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("proxyArpConfigurator Initialized")

	// Route configurator
	plugin.routeConfigurator = &l3plugin.RouteConfigurator{}
	if err := plugin.routeConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("routeConfigurator Initialized")

	return nil
}

func (plugin *Plugin) initL4(ctx context.Context) error {
	plugin.Log.Infof("Init L4 plugin")

	// Application namespace conifgurator
	plugin.appNsConfigurator = &l4plugin.AppNsConfigurator{}
	if err := plugin.appNsConfigurator.Init(plugin.Log, plugin.GoVppmux, plugin.swIfIndexes, plugin.enableStopwatch); err != nil {
		return err
	}
	plugin.Log.Debug("l4Configurator Initialized")

	return nil
}

func (plugin *Plugin) initSR(ctx context.Context) (err error) {
	plugin.Log.Infof("Init SR plugin")

	// logger
	srLogger := plugin.Log.NewLogger("-sr-plugin")

	var stopwatch *measure.Stopwatch
	if plugin.enableStopwatch {
		stopwatch = measure.NewStopwatch("SRConfigurator", srLogger)
	}
	// configuring configurators
	plugin.srv6Configurator = &srplugin.SRv6Configurator{
		Log:         srLogger,
		GoVppmux:    plugin.GoVppmux,
		SwIfIndexes: plugin.swIfIndexes,
		VppCalls:    vppcalls.NewSRv6Calls(srLogger, stopwatch),
	}
	// Init SR plugin
	if err := plugin.srv6Configurator.Init(); err != nil {
		return err
	}

	plugin.Log.Debug("SRConfigurator Initialized")
	return nil
}

func (plugin *Plugin) initErrorHandler() error {
	ehLogger := plugin.Log.NewLogger("-error-handler")
	plugin.errorIndexes = nametoidx.NewNameToIdx(ehLogger, "error_indexes", nil)

	// Init mapping index
	plugin.errorIdxSeq = 1
	return nil
}

func (plugin *Plugin) fromConfigFile() {
	config, err := plugin.retrieveVPPConfig()
	if err != nil {
		plugin.Log.Errorf("Error reading vpp-plugin config file: %v", err)
		return
	}
	if config != nil {
		publishers := &datasync.CompositeKVProtoWriter{}
		for _, pub := range config.StatusPublishers {
			db, found := plugin.Deps.DataSyncs[pub]
			if !found {
				plugin.Log.Warnf("Unknown status publisher %q from config", pub)
				continue
			}
			publishers.Adapters = append(publishers.Adapters, db)
			plugin.Log.Infof("Added status publisher %q from config", pub)
		}
		plugin.Deps.PublishStatistics = publishers
		if config.Mtu != 0 {
			plugin.ifMtu = config.Mtu
			plugin.Log.Infof("Default MTU set to %v", plugin.ifMtu)
		}
		plugin.enableStopwatch = config.Stopwatch
		if plugin.enableStopwatch {
			plugin.Log.Infof("stopwatch enabled for %v", plugin.PluginName)
		} else {
			plugin.Log.Infof("stopwatch disabled for %v", plugin.PluginName)
		}
		// return skip (if set) or value from config
		plugin.resyncStrategy = plugin.resolveResyncStrategy(config.Strategy)
		plugin.Log.Infof("VPP resync strategy is set to %v", plugin.resyncStrategy)
	} else {
		plugin.Log.Infof("stopwatch disabled for %v", plugin.PluginName)
		// return skip (if set) or full
		plugin.resyncStrategy = plugin.resolveResyncStrategy(fullResync)
		plugin.Log.Infof("VPP resync strategy config not found, set to %v", plugin.resyncStrategy)
	}
}

func (plugin *Plugin) retrieveVPPConfig() (*PluginConfig, error) {
	config := &PluginConfig{}

	found, err := plugin.PluginConfig.GetValue(config)
	if err != nil {
		return nil, err
	} else if !found {
		plugin.Log.Debug("vpp-plugin config not found")
		return nil, nil
	}
	plugin.Log.Debugf("vpp-plugin config found: %+v", config)

	if pubs := os.Getenv("VPP_STATUS_PUBLISHERS"); pubs != "" {
		plugin.Log.Debugf("status publishers from env: %v", pubs)
		config.StatusPublishers = append(config.StatusPublishers, pubs)
	}

	return config, err
}
