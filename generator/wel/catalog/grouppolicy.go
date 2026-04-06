package catalog

import (
	"fmt"
	"math/rand"
)

const groupPolicyChannel = "Microsoft-Windows-GroupPolicy/Operational"

func init() {
	gpProvider := "Microsoft-Windows-GroupPolicy"
	gpGUID := "{aea1b4fa-97d1-45f2-a64c-4d69fffd92c9}"

	gpEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4016, LevelInformation, generateGPOProcessingStarted},
		{5016, LevelInformation, generateGPOProcessingCompleted},
		{5312, LevelInformation, generateGPOListRetrieved},
		{7016, LevelWarning, generateGPOProcessingFailed},
		{8001, LevelInformation, generateGPOApplied},
	}

	for _, ev := range gpEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      groupPolicyChannel,
			Provider:     gpProvider,
			ProviderGUID: gpGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleDC,
			Generate:     ev.gen,
		})
	}
}

func generateGPOProcessingStarted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "PrincipalSamName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "IsMachine", Value: "false"},
		{Name: "IsBackgroundProcessing", Value: "true"},
	}
	return data, fmt.Sprintf("Starting Group Policy processing for user %s\\%s.", opts.DomainName, user)
}

func generateGPOProcessingCompleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "PrincipalSamName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
		{Name: "IsMachine", Value: "false"},
		{Name: "IsBackgroundProcessing", Value: "true"},
		{Name: "ProcessingTimeInMilliseconds", Value: fmt.Sprintf("%d", r.Intn(5000)+500)}, // #nosec G404
	}
	return data, fmt.Sprintf("Completed Group Policy processing for user %s\\%s.", opts.DomainName, user)
}

func generateGPOListRetrieved(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	gpoNames := []string{"Default Domain Policy", "Default Domain Controllers Policy", "Security Baseline", "Software Installation"}
	gpo := gpoNames[r.Intn(len(gpoNames))] // #nosec G404
	data := []EventDataField{
		{Name: "GPOName", Value: gpo},
		{Name: "GPOLink", Value: "DC=" + opts.DomainName},
		{Name: "SOMOrder", Value: "1"},
	}
	return data, fmt.Sprintf("List of applicable Group Policy objects retrieved: %s", gpo)
}

func generateGPOProcessingFailed(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	extensions := []string{"Registry", "Security", "Scripts", "Folder Redirection"}
	ext := extensions[r.Intn(len(extensions))] // #nosec G404
	data := []EventDataField{
		{Name: "ExtensionName", Value: ext},
		{Name: "ErrorCode", Value: "1030"},
		{Name: "ErrorDescription", Value: "The processing of Group Policy failed because of network problems."},
	}
	_ = opts
	return data, fmt.Sprintf("The processing of Group Policy extension %s failed. Error: 1030.", ext)
}

func generateGPOApplied(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	gpoNames := []string{"Default Domain Policy", "Default Domain Controllers Policy", "Security Baseline"}
	gpo := gpoNames[r.Intn(len(gpoNames))] // #nosec G404
	data := []EventDataField{
		{Name: "GPOName", Value: gpo},
		{Name: "IsComputer", Value: "true"},
	}
	_ = opts
	return data, fmt.Sprintf("Completed applying Group Policy object %s to the computer.", gpo)
}
