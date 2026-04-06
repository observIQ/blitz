package catalog

import (
	"fmt"
	"math/rand"
)

const setupChannel = "Setup"

func init() {
	setupProvider := "Microsoft-Windows-Servicing"
	setupGUID := "{bd12f3b8-fc40-4a61-a307-b7a013a069c1}"

	Register(EventDefinition{
		Channel:      setupChannel,
		Provider:     setupProvider,
		ProviderGUID: setupGUID,
		EventID:      1,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			kbs := []string{"KB5034441", "KB5033372", "KB5032190", "KB5031356", "KB5030219"}
			kb := kbs[r.Intn(len(kbs))] // #nosec G404
			data := []EventDataField{
				{Name: "PackageName", Value: fmt.Sprintf("Package_%s~31bf3856ad364e35~amd64~~10.0.1.1", kb)},
				{Name: "PackageState", Value: "Installed"},
			}
			return data, fmt.Sprintf("Windows update %s was successfully installed.", kb)
		},
	})

	Register(EventDefinition{
		Channel:      setupChannel,
		Provider:     setupProvider,
		ProviderGUID: setupGUID,
		EventID:      2,
		Level:        LevelError,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			kbs := []string{"KB5034441", "KB5033372"}
			kb := kbs[r.Intn(len(kbs))] // #nosec G404
			data := []EventDataField{
				{Name: "PackageName", Value: fmt.Sprintf("Package_%s~31bf3856ad364e35~amd64~~10.0.1.1", kb)},
				{Name: "ErrorCode", Value: "0x800f0922"},
			}
			return data, fmt.Sprintf("Windows update %s failed to install with error 0x800f0922.", kb)
		},
	})

	Register(EventDefinition{
		Channel:      setupChannel,
		Provider:     "Microsoft-Windows-WindowsUpdateClient",
		ProviderGUID: "{945a8954-c147-4acd-923f-40c45405a658}",
		EventID:      19,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			kbs := []string{"KB5034441", "KB5033372", "KB5032190"}
			kb := kbs[r.Intn(len(kbs))] // #nosec G404
			data := []EventDataField{
				{Name: "updateTitle", Value: fmt.Sprintf("2024-03 Cumulative Update for Windows (%s)", kb)},
				{Name: "updateGuid", Value: RandomGUID(r)},
			}
			return data, fmt.Sprintf("Installation Successful: Windows successfully installed the following update: 2024-03 Cumulative Update for Windows (%s)", kb)
		},
	})

	Register(EventDefinition{
		Channel:      setupChannel,
		Provider:     "Microsoft-Windows-WindowsUpdateClient",
		ProviderGUID: "{945a8954-c147-4acd-923f-40c45405a658}",
		EventID:      20,
		Level:        LevelError,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			kbs := []string{"KB5034441", "KB5033372"}
			kb := kbs[r.Intn(len(kbs))] // #nosec G404
			data := []EventDataField{
				{Name: "updateTitle", Value: fmt.Sprintf("2024-03 Cumulative Update for Windows (%s)", kb)},
				{Name: "updateGuid", Value: RandomGUID(r)},
				{Name: "errorCode", Value: "0x80070005"},
			}
			return data, fmt.Sprintf("Installation Failure: Windows failed to install the following update with error 0x80070005: 2024-03 Cumulative Update for Windows (%s)", kb)
		},
	})
}
