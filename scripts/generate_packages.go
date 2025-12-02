// Package generator creates Package wrappers for existing services.
// This tool automates the migration to the Android-style package model.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// ServiceInfo contains metadata for generating a package.
type ServiceInfo struct {
	PackageName  string // Go package name (e.g., "accounts")
	PackageID    string // Android-style ID (e.g., "com.r3e.services.accounts")
	DisplayName  string // Human-readable name
	Description  string // Service description
	Domain       string // Service domain
	Capabilities []string
	StorageMB    int // Storage quota in MB
	MaxRPS       int // Max requests per second
	MaxEvents    int // Max events per second
}

var packageTemplate = `// Package {{.PackageName}} provides the {{.DisplayName}} as a ServicePackage.
package {{.PackageName}}

import (
	"context"

	"github.com/R3E-Network/service_layer/pkg/storage"
	engine "github.com/R3E-Network/service_layer/system/engine/core"
	pkg "github.com/R3E-Network/service_layer/system/os/runtime"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Package implements the ServicePackage interface.
type Package struct{}

func init() {
	pkg.MustRegisterPackage("{{.PackageID}}", func() (pkg.ServicePackage, error) {
		return &Package{}, nil
	})
}

func (p *Package) Manifest() pkg.PackageManifest {
	return pkg.PackageManifest{
		PackageID:   "{{.PackageID}}",
		Version:     "1.0.0",
		DisplayName: "{{.DisplayName}}",
		Description: "{{.Description}}",
		Author:      "R3E Network",
		License:     "MIT",

		Services: []pkg.ServiceDeclaration{
			{
				Name:         "{{.PackageName}}",
				Domain:       "{{.Domain}}",
				Description:  "{{.Description}}",
				Capabilities: []string{ {{range .Capabilities}}"{{.}}", {{end}} },
				Layer:        "service",
			},
		},

		Permissions: []pkg.Permission{
			{
				Name:        "engine.api.storage",
				Description: "Required for data persistence",
				Required:    true,
			},
			{
				Name:        "engine.api.bus",
				Description: "Required for event publishing",
				Required:    false,
			},
		},

		Resources: pkg.ResourceQuotas{
			MaxStorageBytes:       {{.StorageMB}} * 1024 * 1024,
			MaxConcurrentRequests: 1000,
			MaxRequestsPerSecond:  {{.MaxRPS}},
			MaxEventsPerSecond:    {{.MaxEvents}},
		},

		Dependencies: []pkg.Dependency{
			{
				EngineModule: "store",
				Required:     true,
			},
		},
	}
}

func (p *Package) CreateServices(ctx context.Context, runtime pkg.PackageRuntime) ([]engine.ServiceModule, error) {
	_ = ctx
	// TODO: Adapt PackageStorage to service-specific storage interface
	// For now, this maintains compatibility with existing service constructor
	var store storage.Store // Placeholder

	log := logger.NewDefault("{{.PackageName}}")
	if loggerFromRuntime := runtime.Logger(); loggerFromRuntime != nil {
		if l, ok := loggerFromRuntime.(*logger.Logger); ok {
			log = l
		}
	}

	svc := New(store, log)
	return []engine.ServiceModule{svc}, nil
}

func (p *Package) OnInstall(ctx context.Context, runtime pkg.PackageRuntime) error {
	_ = ctx
	if log := runtime.Logger(); log != nil {
		if l, ok := log.(*logger.Logger); ok {
			l.Info("{{.PackageName}} package installed")
		}
	}
	return nil
}

func (p *Package) OnUninstall(ctx context.Context, runtime pkg.PackageRuntime) error {
	_ = ctx
	if log := runtime.Logger(); log != nil {
		if l, ok := log.(*logger.Logger); ok {
			l.Info("{{.PackageName}} package uninstalled")
		}
	}
	return nil
}

func (p *Package) OnUpgrade(ctx context.Context, runtime pkg.PackageRuntime, oldVersion string) error {
	_ = ctx
	if log := runtime.Logger(); log != nil {
		if l, ok := log.(*logger.Logger); ok {
			l.WithField("old_version", oldVersion).
				WithField("new_version", p.Manifest().Version).
				Info("{{.PackageName}} package upgraded")
		}
	}
	return nil
}
`

// Services to migrate
var services = []ServiceInfo{
	{
		PackageName:  "vrf",
		PackageID:    "com.r3e.services.vrf",
		DisplayName:  "VRF Service",
		Description:  "Verifiable Random Function service",
		Domain:       "vrf",
		Capabilities: []string{"vrf.request", "vrf.verify"},
		StorageMB:    100,
		MaxRPS:       5000,
		MaxEvents:    1000,
	},
	{
		PackageName:  "oracle",
		PackageID:    "com.r3e.services.oracle",
		DisplayName:  "Oracle Service",
		Description:  "Decentralized oracle data feeds",
		Domain:       "oracle",
		Capabilities: []string{"oracle.request", "oracle.fulfill"},
		StorageMB:    200,
		MaxRPS:       10000,
		MaxEvents:    2000,
	},
	{
		PackageName:  "triggers",
		PackageID:    "com.r3e.services.triggers",
		DisplayName:  "Triggers Service",
		Description:  "Event-driven trigger management",
		Domain:       "triggers",
		Capabilities: []string{"triggers.create", "triggers.fire"},
		StorageMB:    150,
		MaxRPS:       8000,
		MaxEvents:    3000,
	},
	{
		PackageName:  "gasbank",
		PackageID:    "com.r3e.services.gasbank",
		DisplayName:  "Gas Bank Service",
		Description:  "Gas fee management and sponsorship",
		Domain:       "gasbank",
		Capabilities: []string{"gasbank.deposit", "gasbank.withdraw"},
		StorageMB:    50,
		MaxRPS:       3000,
		MaxEvents:    500,
	},
	{
		PackageName:  "automation",
		PackageID:    "com.r3e.services.automation",
		DisplayName:  "Automation Service",
		Description:  "Automated task scheduling and execution",
		Domain:       "automation",
		Capabilities: []string{"automation.schedule", "automation.execute"},
		StorageMB:    300,
		MaxRPS:       5000,
		MaxEvents:    2000,
	},
	{
		PackageName:  "pricefeed",
		PackageID:    "com.r3e.services.pricefeed",
		DisplayName:  "Price Feed Service",
		Description:  "Real-time price data aggregation",
		Domain:       "pricefeed",
		Capabilities: []string{"price.get", "price.subscribe"},
		StorageMB:    100,
		MaxRPS:       15000,
		MaxEvents:    5000,
	},
	{
		PackageName:  "datafeeds",
		PackageID:    "com.r3e.services.datafeeds",
		DisplayName:  "Data Feeds Service",
		Description:  "Generic data feed subscriptions",
		Domain:       "datafeeds",
		Capabilities: []string{"feed.subscribe", "feed.publish"},
		StorageMB:    200,
		MaxRPS:       10000,
		MaxEvents:    3000,
	},
	{
		PackageName:  "datastreams",
		PackageID:    "com.r3e.services.datastreams",
		DisplayName:  "Data Streams Service",
		Description:  "Real-time data streaming",
		Domain:       "datastreams",
		Capabilities: []string{"stream.publish", "stream.subscribe"},
		StorageMB:    150,
		MaxRPS:       12000,
		MaxEvents:    4000,
	},
	{
		PackageName:  "datalink",
		PackageID:    "com.r3e.services.datalink",
		DisplayName:  "Data Link Service",
		Description:  "Cross-chain data linking",
		Domain:       "datalink",
		Capabilities: []string{"link.create", "link.query"},
		StorageMB:    100,
		MaxRPS:       5000,
		MaxEvents:    1000,
	},
	{
		PackageName:  "dta",
		PackageID:    "com.r3e.services.dta",
		DisplayName:  "DTA Service",
		Description:  "Decentralized Token Automation",
		Domain:       "dta",
		Capabilities: []string{"dta.create", "dta.execute"},
		StorageMB:    100,
		MaxRPS:       5000,
		MaxEvents:    1000,
	},
	{
		PackageName:  "confidential",
		PackageID:    "com.r3e.services.confidential",
		DisplayName:  "Confidential Service",
		Description:  "Confidential computing and privacy",
		Domain:       "confidential",
		Capabilities: []string{"confidential.encrypt", "confidential.decrypt"},
		StorageMB:    50,
		MaxRPS:       3000,
		MaxEvents:    500,
	},
	{
		PackageName:  "secrets",
		PackageID:    "com.r3e.services.secrets",
		DisplayName:  "Secrets Service",
		Description:  "Secret management and key rotation",
		Domain:       "secrets",
		Capabilities: []string{"secrets.store", "secrets.retrieve"},
		StorageMB:    50,
		MaxRPS:       2000,
		MaxEvents:    200,
	},
	{
		PackageName:  "random",
		PackageID:    "com.r3e.services.random",
		DisplayName:  "Random Service",
		Description:  "Cryptographically secure random number generation",
		Domain:       "random",
		Capabilities: []string{"random.generate"},
		StorageMB:    20,
		MaxRPS:       10000,
		MaxEvents:    1000,
	},
	{
		PackageName:  "ccip",
		PackageID:    "com.r3e.services.ccip",
		DisplayName:  "CCIP Service",
		Description:  "Cross-Chain Interoperability Protocol",
		Domain:       "ccip",
		Capabilities: []string{"ccip.send", "ccip.receive"},
		StorageMB:    200,
		MaxRPS:       5000,
		MaxEvents:    2000,
	},
	{
		PackageName:  "cre",
		PackageID:    "com.r3e.services.cre",
		DisplayName:  "CRE Service",
		Description:  "Contract Runtime Environment",
		Domain:       "cre",
		Capabilities: []string{"cre.execute", "cre.deploy"},
		StorageMB:    500,
		MaxRPS:       8000,
		MaxEvents:    3000,
	},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run generate_packages.go <services_dir>")
		fmt.Println("Example: go run generate_packages.go internal/services")
		os.Exit(1)
	}

	servicesDir := os.Args[1]

	tmpl, err := template.New("package").Parse(packageTemplate)
	if err != nil {
		fmt.Printf("Failed to parse template: %v\n", err)
		os.Exit(1)
	}

	for _, svc := range services {
		outputPath := filepath.Join(servicesDir, svc.PackageName, "package.go")

		// Check if directory exists
		dir := filepath.Dir(outputPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Printf("Directory %s does not exist, skipping %s\n", dir, svc.PackageName)
			continue
		}

		// Create file
		f, err := os.Create(outputPath)
		if err != nil {
			fmt.Printf("Failed to create %s: %v\n", outputPath, err)
			continue
		}

		// Execute template
		if err := tmpl.Execute(f, svc); err != nil {
			fmt.Printf("Failed to generate %s: %v\n", outputPath, err)
			f.Close()
			continue
		}

		f.Close()
		fmt.Printf("✓ Generated %s\n", outputPath)
	}

	fmt.Println("\nPackage generation complete!")
	fmt.Println("Next steps:")
	fmt.Println("1. Review generated package.go files")
	fmt.Println("2. Adjust storage adapters in CreateServices()")
	fmt.Println("3. Update service constructors if needed")
	fmt.Println("4. Run: go test ./internal/services/...")
}
