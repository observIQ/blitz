package catalog

import (
	"fmt"
	"math/rand"
)

const powershellChannel = "Microsoft-Windows-PowerShell/Operational"

var scriptBlocks = []string{
	`Get-Process | Where-Object { $_.CPU -gt 100 }`,
	`Get-EventLog -LogName Security -Newest 100`,
	`Invoke-WebRequest -Uri "https://update.contoso.com/check" -Method GET`,
	`Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser`,
	`Get-ADUser -Filter * -Properties LastLogonDate | Sort-Object LastLogonDate`,
	`$service = Get-Service -Name 'wuauserv'; Restart-Service $service`,
	`Import-Module ActiveDirectory; Get-ADComputer -Filter *`,
	`New-Item -Path 'C:\Logs' -ItemType Directory -Force`,
	`[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.SecurityProtocolType]::Tls12`,
}

func init() {
	psProvider := "Microsoft-Windows-PowerShell"
	psGUID := "{a0c1853b-5c40-4b15-8766-3cf1c58f985a}"

	// 4104 - Script block logging
	Register(EventDefinition{
		Channel:      powershellChannel,
		Provider:     psProvider,
		ProviderGUID: psGUID,
		EventID:      4104,
		Level:        LevelVerbose,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			script := scriptBlocks[r.Intn(len(scriptBlocks))] // #nosec G404
			data := []EventDataField{
				{Name: "MessageNumber", Value: "1"},
				{Name: "MessageTotal", Value: "1"},
				{Name: "ScriptBlockText", Value: script},
				{Name: "ScriptBlockId", Value: RandomGUID(r)},
				{Name: "Path", Value: ""},
			}
			return data, fmt.Sprintf("Creating Scriptblock text (%d characters):\n%s", len(script), script)
		},
	})

	// 4103 - Module logging
	Register(EventDefinition{
		Channel:      powershellChannel,
		Provider:     psProvider,
		ProviderGUID: psGUID,
		EventID:      4103,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			cmds := []string{"Get-Process", "Get-Service", "Get-EventLog", "Get-ADUser", "Invoke-WebRequest"}
			cmd := cmds[r.Intn(len(cmds))] // #nosec G404
			user := PickUsername(r, opts.Usernames)
			data := []EventDataField{
				{Name: "ContextInfo", Value: fmt.Sprintf("Severity = Informational\n\tHost Name = ConsoleHost\n\tHost Version = 5.1.19041.1\n\tHost ID = %s\n\tHost Application = C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe\n\tEngine Version = 5.1.19041.1\n\tRunspace ID = %s\n\tPipeline ID = 1\n\tCommand Name = %s\n\tCommand Type = Cmdlet\n\tScript Name = \n\tCommand Path = \n\tSequence Number = 1\n\tUser = %s\\%s\n\tConnected User = \n\tShell ID = Microsoft.PowerShell",
					RandomGUID(r), RandomGUID(r), cmd, opts.DomainName, user)},
				{Name: "Payload", Value: fmt.Sprintf("CommandInvocation(%s): \"%s\"", cmd, cmd)},
			}
			return data, fmt.Sprintf("CommandInvocation(%s): \"%s\"\nUser = %s\\%s", cmd, cmd, opts.DomainName, user)
		},
	})

	// 40961 - PowerShell starting up
	Register(EventDefinition{
		Channel:      powershellChannel,
		Provider:     psProvider,
		ProviderGUID: psGUID,
		EventID:      40961,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			data := []EventDataField{
				{Name: "HostId", Value: RandomGUID(r)},
				{Name: "HostApplication", Value: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
			}
			return data, "PowerShell console is starting up."
		},
	})

	// 40962 - PowerShell ready for input
	Register(EventDefinition{
		Channel:      powershellChannel,
		Provider:     psProvider,
		ProviderGUID: psGUID,
		EventID:      40962,
		Level:        LevelInformation,
		MinRole:      RoleWorkstation,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			data := []EventDataField{
				{Name: "HostId", Value: RandomGUID(r)},
			}
			return data, "PowerShell console is ready for user input."
		},
	})
}
