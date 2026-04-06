package catalog

import (
	"fmt"
	"math/rand"
)

const (
	taskUserAccount     = 13824
	taskGroupMgmt       = 13826
	taskDomainGroupMgmt = 13827
	taskLocalGroupMgmt  = 13828
	taskComputerAccount = 13830
)

func init() {
	// User account management events
	for _, ev := range []struct {
		id      int
		task    int
		kw      uint64
		genFunc func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4720, taskUserAccount, keywordsAuditSuccess, generateUserAccountCreated},
		{4722, taskUserAccount, keywordsAuditSuccess, generateUserAccountEnabled},
		{4723, taskUserAccount, keywordsAuditSuccess, generatePasswordChange},
		{4724, taskUserAccount, keywordsAuditSuccess, generatePasswordReset},
		{4725, taskUserAccount, keywordsAuditSuccess, generateUserAccountDisabled},
		{4726, taskUserAccount, keywordsAuditSuccess, generateUserAccountDeleted},
		{4738, taskUserAccount, keywordsAuditSuccess, generateUserAccountChanged},
		{4740, taskUserAccount, keywordsAuditFailure, generateAccountLockedOut},
		{4767, taskUserAccount, keywordsAuditSuccess, generateAccountUnlocked},
		{4781, taskUserAccount, keywordsAuditSuccess, generateAccountNameChanged},
		{4798, taskUserAccount, keywordsAuditSuccess, generateGroupMembershipEnum},
		{4799, taskUserAccount, keywordsAuditSuccess, generateLocalGroupMembershipEnum},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         ev.task,
			TaskName:     "User Account Management",
			Keywords:     ev.kw,
			KeywordNames: keywordNames(ev.kw),
			MinRole:      RoleWorkstation,
			Generate:     ev.genFunc,
		})
	}

	// Security-enabled global group events
	for _, ev := range []struct {
		id   int
		desc string
		gen  func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4727, "created", generateGlobalGroupCreated},
		{4728, "member added", generateGlobalGroupMemberAdded},
		{4729, "member removed", generateGlobalGroupMemberRemoved},
		{4730, "deleted", generateGlobalGroupDeleted},
		{4737, "changed", generateGlobalGroupChanged},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         taskDomainGroupMgmt,
			TaskName:     "Security Group Management",
			Keywords:     keywordsAuditSuccess,
			KeywordNames: []string{"Audit Success"},
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}

	// Security-enabled local group events
	for _, ev := range []struct {
		id  int
		gen func(*rand.Rand, *GenerateOpts) ([]EventDataField, string)
	}{
		{4731, generateLocalGroupCreated},
		{4732, generateLocalGroupMemberAdded},
		{4733, generateLocalGroupMemberRemoved},
		{4734, generateLocalGroupDeleted},
		{4735, generateLocalGroupChanged},
	} {
		ev := ev
		Register(EventDefinition{
			Channel:      "Security",
			Provider:     securityProvider,
			ProviderGUID: securityProviderGUID,
			EventID:      ev.id,
			Level:        LevelLogAlways,
			Task:         taskLocalGroupMgmt,
			TaskName:     "Security Group Management",
			Keywords:     keywordsAuditSuccess,
			KeywordNames: []string{"Audit Success"},
			MinRole:      RoleWorkstation,
			Generate:     ev.gen,
		})
	}
}

func keywordNames(kw uint64) []string {
	switch kw {
	case keywordsAuditSuccess:
		return []string{"Audit Success"}
	case keywordsAuditFailure:
		return []string{"Audit Failure"}
	default:
		return nil
	}
}

func subjectFields(r *rand.Rand, opts *GenerateOpts) []EventDataField {
	return []EventDataField{
		{Name: "SubjectUserSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		{Name: "SubjectUserName", Value: PickUsername(r, opts.Usernames)},
		{Name: "SubjectDomainName", Value: opts.DomainName},
		{Name: "SubjectLogonId", Value: RandomLogonID(r)},
	}
}

func generateUserAccountCreated(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "PrivilegeList", Value: "-"},
		EventDataField{Name: "SamAccountName", Value: target},
		EventDataField{Name: "DisplayName", Value: target},
		EventDataField{Name: "UserPrincipalName", Value: target + "@" + opts.DomainName},
		EventDataField{Name: "HomeDirectory", Value: "-"},
		EventDataField{Name: "HomePath", Value: "-"},
		EventDataField{Name: "ScriptPath", Value: "-"},
		EventDataField{Name: "ProfilePath", Value: "-"},
		EventDataField{Name: "UserWorkstations", Value: "-"},
		EventDataField{Name: "PasswordLastSet", Value: "-"},
		EventDataField{Name: "AccountExpires", Value: "%%1794"},
		EventDataField{Name: "PrimaryGroupId", Value: "513"},
		EventDataField{Name: "AllowedToDelegateTo", Value: "-"},
		EventDataField{Name: "OldUacValue", Value: "0x0"},
		EventDataField{Name: "NewUacValue", Value: "0x15"},
		EventDataField{Name: "UserAccountControl", Value: "%%2080 %%2082 %%2084"},
		EventDataField{Name: "UserParameters", Value: "-"},
		EventDataField{Name: "SidHistory", Value: "-"},
		EventDataField{Name: "LogonHours", Value: "%%1793"},
	)
	msg := fmt.Sprintf("A user account was created.\n\nSubject:\n\tAccount Name:\t\t%s\n\nNew Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateUserAccountEnabled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("A user account was enabled.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generatePasswordChange(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("An attempt was made to change an account's password.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generatePasswordReset(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("An attempt was made to reset an account's password.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateUserAccountDisabled(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("A user account was disabled.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateUserAccountDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("A user account was deleted.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateUserAccountChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "PrivilegeList", Value: "-"},
		EventDataField{Name: "SamAccountName", Value: target},
		EventDataField{Name: "DisplayName", Value: target},
		EventDataField{Name: "UserPrincipalName", Value: target + "@" + opts.DomainName},
		EventDataField{Name: "HomeDirectory", Value: "-"},
		EventDataField{Name: "HomePath", Value: "-"},
		EventDataField{Name: "ScriptPath", Value: "-"},
		EventDataField{Name: "ProfilePath", Value: "-"},
		EventDataField{Name: "UserWorkstations", Value: "-"},
		EventDataField{Name: "PasswordLastSet", Value: "-"},
		EventDataField{Name: "AccountExpires", Value: "%%1794"},
		EventDataField{Name: "PrimaryGroupId", Value: "513"},
		EventDataField{Name: "AllowedToDelegateTo", Value: "-"},
		EventDataField{Name: "OldUacValue", Value: "0x15"},
		EventDataField{Name: "NewUacValue", Value: "0x11"},
		EventDataField{Name: "UserAccountControl", Value: "%%2080 %%2082"},
		EventDataField{Name: "UserParameters", Value: "-"},
		EventDataField{Name: "SidHistory", Value: "-"},
		EventDataField{Name: "LogonHours", Value: "%%1793"},
	)
	msg := fmt.Sprintf("A user account was changed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateAccountLockedOut(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("A user account was locked out.\n\nSubject:\n\tAccount Name:\t\t%s\n\nAccount That Was Locked Out:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateAccountUnlocked(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
	)
	msg := fmt.Sprintf("A user account was unlocked.\n\nSubject:\n\tAccount Name:\t\t%s\n\nTarget Account:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateAccountNameChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	oldName := PickUsername(r, opts.Usernames)
	newName := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: oldName},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "OldTargetUserName", Value: oldName},
		EventDataField{Name: "NewTargetUserName", Value: newName},
	)
	msg := fmt.Sprintf("The name of an account was changed.\n\nOld Account Name:\t%s\nNew Account Name:\t%s",
		oldName, newName)
	return data, msg
}

func generateGroupMembershipEnum(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "CallerWorkstation", Value: PickHostname(r, opts.Hostnames)},
	)
	msg := fmt.Sprintf("A user's local group membership was enumerated.\n\nSubject:\n\tAccount Name:\t\t%s\n\nUser:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

func generateLocalGroupMembershipEnum(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	target := PickUsername(r, opts.Usernames)
	data := append(subjectFields(r, opts),
		EventDataField{Name: "TargetUserName", Value: target},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "CallerWorkstation", Value: PickHostname(r, opts.Hostnames)},
	)
	msg := fmt.Sprintf("A security-enabled local group membership was enumerated.\n\nSubject:\n\tAccount Name:\t\t%s\n\nGroup:\n\tAccount Name:\t\t%s",
		data[1].Value, target)
	return data, msg
}

// Global group event generators
func groupEventFields(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string, string) {
	groupName := fmt.Sprintf("Group-%d", r.Intn(100)) // #nosec G404
	groupSID := RandomSID(r, "S-1-5-21-0-0-0")
	subject := subjectFields(r, opts)
	return subject, groupName, groupSID
}

func generateGlobalGroupCreated(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	data := append(subject,
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A security-enabled global group was created.\n\nSubject:\n\tAccount Name:\t\t%s\n\nNew Group:\n\tGroup Name:\t\t%s",
		data[1].Value, groupName)
	return data, msg
}

func generateGlobalGroupMemberAdded(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	member := PickUsername(r, opts.Usernames)
	data := append(subject,
		EventDataField{Name: "MemberName", Value: fmt.Sprintf("CN=%s,CN=Users,%s", member, "DC="+opts.DomainName)},
		EventDataField{Name: "MemberSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A member was added to a security-enabled global group.\n\nSubject:\n\tAccount Name:\t\t%s\n\nMember:\n\tAccount Name:\t\t%s\n\nGroup:\n\tGroup Name:\t\t%s",
		data[1].Value, member, groupName)
	return data, msg
}

func generateGlobalGroupMemberRemoved(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	member := PickUsername(r, opts.Usernames)
	data := append(subject,
		EventDataField{Name: "MemberName", Value: fmt.Sprintf("CN=%s,CN=Users,%s", member, "DC="+opts.DomainName)},
		EventDataField{Name: "MemberSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A member was removed from a security-enabled global group.\n\nSubject:\n\tAccount Name:\t\t%s\n\nMember:\n\tAccount Name:\t\t%s\n\nGroup:\n\tGroup Name:\t\t%s",
		data[1].Value, member, groupName)
	return data, msg
}

func generateGlobalGroupDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	data := append(subject,
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A security-enabled global group was deleted.\n\nSubject:\n\tAccount Name:\t\t%s\n\nGroup:\n\tGroup Name:\t\t%s",
		data[1].Value, groupName)
	return data, msg
}

func generateGlobalGroupChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	data := append(subject,
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: opts.DomainName},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A security-enabled global group was changed.\n\nSubject:\n\tAccount Name:\t\t%s\n\nGroup:\n\tGroup Name:\t\t%s",
		data[1].Value, groupName)
	return data, msg
}

// Local group event generators
func generateLocalGroupCreated(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	data := append(subject,
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: "Builtin"},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A security-enabled local group was created.\n\nGroup Name:\t%s", groupName)
	return data, msg
}

func generateLocalGroupMemberAdded(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	member := PickUsername(r, opts.Usernames)
	data := append(subject,
		EventDataField{Name: "MemberName", Value: fmt.Sprintf("CN=%s,CN=Users,%s", member, "DC="+opts.DomainName)},
		EventDataField{Name: "MemberSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: "Builtin"},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A member was added to a security-enabled local group.\n\nMember:\n\tAccount Name:\t\t%s\n\nGroup:\n\tGroup Name:\t\t%s",
		member, groupName)
	return data, msg
}

func generateLocalGroupMemberRemoved(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	member := PickUsername(r, opts.Usernames)
	data := append(subject,
		EventDataField{Name: "MemberName", Value: fmt.Sprintf("CN=%s,CN=Users,%s", member, "DC="+opts.DomainName)},
		EventDataField{Name: "MemberSid", Value: RandomSID(r, "S-1-5-21-0-0-0")},
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: "Builtin"},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A member was removed from a security-enabled local group.\n\nMember:\n\tAccount Name:\t\t%s\n\nGroup:\n\tGroup Name:\t\t%s",
		member, groupName)
	return data, msg
}

func generateLocalGroupDeleted(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	data := append(subject,
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: "Builtin"},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A security-enabled local group was deleted.\n\nGroup Name:\t%s", groupName)
	return data, msg
}

func generateLocalGroupChanged(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
	subject, groupName, groupSID := groupEventFields(r, opts)
	data := append(subject,
		EventDataField{Name: "TargetUserName", Value: groupName},
		EventDataField{Name: "TargetDomainName", Value: "Builtin"},
		EventDataField{Name: "TargetSid", Value: groupSID},
		EventDataField{Name: "PrivilegeList", Value: "-"},
	)
	msg := fmt.Sprintf("A security-enabled local group was changed.\n\nGroup Name:\t%s", groupName)
	return data, msg
}
