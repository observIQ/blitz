package catalog

import (
	"fmt"
	"math/rand"
)

const taskSchedulerChannel = "Microsoft-Windows-TaskScheduler/Operational"

var taskNames = []string{
	`\Microsoft\Windows\UpdateOrchestrator\Schedule Scan`,
	`\Microsoft\Windows\WindowsUpdate\Scheduled Start`,
	`\Microsoft\Windows\Defrag\ScheduledDefrag`,
	`\Microsoft\Windows\DiskCleanup\SilentCleanup`,
	`\Microsoft\Windows\Maintenance\WinSAT`,
	`\Microsoft\Windows\SystemRestore\SR`,
	`\Microsoft\Windows\TaskScheduler\Regular Maintenance`,
	`\CompanyBackup\DailyReport`,
}

func init() {
	tsProvider := "Microsoft-Windows-TaskScheduler"
	tsGUID := "{de7b24ea-73c8-4a09-985d-5bdadcfa9017}"

	taskEvents := []struct {
		id    int
		level EventLevel
		gen   func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{100, LevelInformation, generateTaskStarted},
		{101, LevelError, generateTaskStartFailed},
		{102, LevelInformation, generateTaskCompleted},
		{106, LevelInformation, generateTaskRegistered},
		{107, LevelInformation, generateTaskTriggered},
		{110, LevelInformation, generateTaskTriggeredByUser},
		{111, LevelInformation, generateTaskTerminated},
		{141, LevelInformation, generateTaskDeleted},
		{142, LevelInformation, generateTaskDisabled},
	}

	for _, ev := range taskEvents {
		ev := ev
		Register(EventDefinition{
			Channel:      taskSchedulerChannel,
			Provider:     tsProvider,
			ProviderGUID: tsGUID,
			EventID:      ev.id,
			Level:        ev.level,
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func generateTaskStarted(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "InstanceId", Value: RandomGUID(r)},
	}
	return data, fmt.Sprintf("Task Scheduler started \"%s\" instance.", task)
}

func generateTaskStartFailed(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "ResultCode", Value: "2147942402"},
	}
	return data, fmt.Sprintf("Task Scheduler failed to start \"%s\". Error value: 2147942402.", task)
}

func generateTaskCompleted(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "InstanceId", Value: RandomGUID(r)},
		{Name: "ResultCode", Value: "0"},
	}
	return data, fmt.Sprintf("Task Scheduler successfully completed task \"%s\", instance %s, action \"%s\".", task, data[1].Value, task)
}

func generateTaskRegistered(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "UserName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Task \"%s\" registered by user \"%s\\%s\".", task, opts.DomainName, user)
}

func generateTaskTriggered(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "InstanceId", Value: RandomGUID(r)},
	}
	return data, fmt.Sprintf("Task Scheduler launched task \"%s\" on scheduler.", task)
}

func generateTaskTriggeredByUser(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "InstanceId", Value: RandomGUID(r)},
		{Name: "UserName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Task Scheduler launched \"%s\" due to user \"%s\\%s\" request.", task, opts.DomainName, user)
}

func generateTaskTerminated(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "InstanceId", Value: RandomGUID(r)},
		{Name: "UserName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("Task Scheduler terminated \"%s\" instance as requested by user \"%s\\%s\".", task, opts.DomainName, user)
}

func generateTaskDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "UserName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("User \"%s\\%s\" deleted Task Scheduler task \"%s\".", opts.DomainName, user, task)
}

func generateTaskDisabled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	task := taskNames[r.Intn(len(taskNames))] // #nosec G404
	user := PickUsername(r, opts.Usernames)
	data := []EventDataField{
		{Name: "TaskName", Value: task},
		{Name: "UserName", Value: fmt.Sprintf(`%s\%s`, opts.DomainName, user)},
	}
	return data, fmt.Sprintf("User \"%s\\%s\" disabled Task Scheduler task \"%s\".", opts.DomainName, user, task)
}
