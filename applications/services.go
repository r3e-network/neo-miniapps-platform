package app

import (
	"github.com/R3E-Network/service_layer/applications/system"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts/service"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation/service"
	ccipsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.ccip/service"
	confsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.confidential/service"
	cresvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.cre/service"
	datafeedsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds/service"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink/service"
	datastreamsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams/service"
	dtasvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.dta/service"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank/service"
	mixersvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.mixer/service"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle/service"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets/service"
	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf/service"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// ServiceProvider exposes service pointers required by transport layers.
type ServiceProvider interface {
	AccountsService() *accounts.Service
	AutomationService() *automationsvc.Service
	VRFService() *vrfsvc.Service
	SecretsService() *secrets.Service
	CCIPService() *ccipsvc.Service
	DataLinkService() *datalinksvc.Service
	DataStreamsService() *datastreamsvc.Service
	DataFeedsService() *datafeedsvc.Service
	GasBankService() *gasbanksvc.Service
	ConfidentialService() *confsvc.Service
	DTAService() *dtasvc.Service
	CREService() *cresvc.Service
	OracleService() *oraclesvc.Service
	MixerService() *mixersvc.Service
	OracleRunnerTokensValue() []string
	DescriptorSnapshot() []core.Descriptor
	// GetServiceRouter returns the ServiceRouter for automatic HTTP endpoint discovery.
	GetServiceRouter() *core.ServiceRouter
}

// ServiceBundle contains all service references shared between Application types.
// Embed this in Application structs to get automatic ServiceProvider implementation.
type ServiceBundle struct {
	Accounts     *accounts.Service
	GasBank      *gasbanksvc.Service
	Automation   *automationsvc.Service
	DataFeeds    *datafeedsvc.Service
	DataStreams  *datastreamsvc.Service
	DataLink     *datalinksvc.Service
	DTA          *dtasvc.Service
	Confidential *confsvc.Service
	Oracle       *oraclesvc.Service
	Secrets      *secrets.Service
	CRE          *cresvc.Service
	CCIP         *ccipsvc.Service
	VRF          *vrfsvc.Service
	Mixer        *mixersvc.Service

	OracleRunnerTokens []string
	AutomationRunner   *automationsvc.Scheduler
	OracleRunner       *oraclesvc.Dispatcher
	GasBankSettlement  system.Service

	ServiceRouter *core.ServiceRouter
}

// ServiceProvider implementation for ServiceBundle
func (b *ServiceBundle) AccountsService() *accounts.Service         { return b.Accounts }
func (b *ServiceBundle) AutomationService() *automationsvc.Service  { return b.Automation }
func (b *ServiceBundle) VRFService() *vrfsvc.Service                { return b.VRF }
func (b *ServiceBundle) SecretsService() *secrets.Service           { return b.Secrets }
func (b *ServiceBundle) CCIPService() *ccipsvc.Service              { return b.CCIP }
func (b *ServiceBundle) DataLinkService() *datalinksvc.Service      { return b.DataLink }
func (b *ServiceBundle) DataStreamsService() *datastreamsvc.Service { return b.DataStreams }
func (b *ServiceBundle) DataFeedsService() *datafeedsvc.Service     { return b.DataFeeds }
func (b *ServiceBundle) GasBankService() *gasbanksvc.Service        { return b.GasBank }
func (b *ServiceBundle) ConfidentialService() *confsvc.Service      { return b.Confidential }
func (b *ServiceBundle) DTAService() *dtasvc.Service                { return b.DTA }
func (b *ServiceBundle) CREService() *cresvc.Service                { return b.CRE }
func (b *ServiceBundle) OracleService() *oraclesvc.Service          { return b.Oracle }
func (b *ServiceBundle) MixerService() *mixersvc.Service            { return b.Mixer }
func (b *ServiceBundle) OracleRunnerTokensValue() []string          { return b.OracleRunnerTokens }
func (b *ServiceBundle) GetServiceRouter() *core.ServiceRouter      { return b.ServiceRouter }

// DescriptorSnapshot must be implemented by embedding types (requires Descriptors() method)
func (a *Application) DescriptorSnapshot() []core.Descriptor       { return a.Descriptors() }
func (a *EngineApplication) DescriptorSnapshot() []core.Descriptor { return a.Descriptors() }
