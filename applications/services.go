package app

import (
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts"
	automationsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.automation"
	ccipsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.ccip"
	confsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.confidential"
	cresvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.cre"
	datafeedsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datafeeds"
	datalinksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datalink"
	datastreamsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.datastreams"
	dtasvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.dta"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.functions"
	gasbanksvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.gasbank"
	mixersvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.mixer"
	oraclesvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.oracle"
	"github.com/R3E-Network/service_layer/packages/com.r3e.services.secrets"
	vrfsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.vrf"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// ServiceProvider exposes service pointers required by transport layers.
type ServiceProvider interface {
	AccountsService() *accounts.Service
	AutomationService() *automationsvc.Service
	VRFService() *vrfsvc.Service
	FunctionsService() *functions.Service
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
}

func (a *Application) AccountsService() *accounts.Service         { return a.Accounts }
func (a *Application) AutomationService() *automationsvc.Service  { return a.Automation }
func (a *Application) VRFService() *vrfsvc.Service                { return a.VRF }
func (a *Application) FunctionsService() *functions.Service       { return a.Functions }
func (a *Application) SecretsService() *secrets.Service           { return a.Secrets }
func (a *Application) CCIPService() *ccipsvc.Service              { return a.CCIP }
func (a *Application) DataLinkService() *datalinksvc.Service      { return a.DataLink }
func (a *Application) DataStreamsService() *datastreamsvc.Service { return a.DataStreams }
func (a *Application) DataFeedsService() *datafeedsvc.Service     { return a.DataFeeds }
func (a *Application) GasBankService() *gasbanksvc.Service        { return a.GasBank }
func (a *Application) ConfidentialService() *confsvc.Service      { return a.Confidential }
func (a *Application) DTAService() *dtasvc.Service                { return a.DTA }
func (a *Application) CREService() *cresvc.Service                { return a.CRE }
func (a *Application) OracleService() *oraclesvc.Service          { return a.Oracle }
func (a *Application) MixerService() *mixersvc.Service            { return a.Mixer }
func (a *Application) OracleRunnerTokensValue() []string          { return a.OracleRunnerTokens }
func (a *Application) DescriptorSnapshot() []core.Descriptor {
	return a.Descriptors()
}

func (a *EngineApplication) AccountsService() *accounts.Service         { return a.Accounts }
func (a *EngineApplication) AutomationService() *automationsvc.Service  { return a.Automation }
func (a *EngineApplication) VRFService() *vrfsvc.Service                { return a.VRF }
func (a *EngineApplication) FunctionsService() *functions.Service       { return a.Functions }
func (a *EngineApplication) SecretsService() *secrets.Service           { return a.Secrets }
func (a *EngineApplication) CCIPService() *ccipsvc.Service              { return a.CCIP }
func (a *EngineApplication) DataLinkService() *datalinksvc.Service      { return a.DataLink }
func (a *EngineApplication) DataStreamsService() *datastreamsvc.Service { return a.DataStreams }
func (a *EngineApplication) DataFeedsService() *datafeedsvc.Service     { return a.DataFeeds }
func (a *EngineApplication) GasBankService() *gasbanksvc.Service        { return a.GasBank }
func (a *EngineApplication) ConfidentialService() *confsvc.Service      { return a.Confidential }
func (a *EngineApplication) DTAService() *dtasvc.Service                { return a.DTA }
func (a *EngineApplication) CREService() *cresvc.Service                { return a.CRE }
func (a *EngineApplication) OracleService() *oraclesvc.Service          { return a.Oracle }
func (a *EngineApplication) MixerService() *mixersvc.Service            { return a.Mixer }
func (a *EngineApplication) OracleRunnerTokensValue() []string          { return a.OracleRunnerTokens }
func (a *EngineApplication) DescriptorSnapshot() []core.Descriptor {
	return a.Descriptors()
}
