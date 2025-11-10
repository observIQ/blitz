package templates

import "strings"

// ExampleTemplateName is the key for the default example Windows Event template.
const ExampleTemplateName = "example"

// exampleXMLTemplate is the Windows Event XML with placeholders.
// It uses {{IP_ADDRESS}} to be replaced at render time.
var exampleXMLTemplate = strings.TrimSpace(`
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='Microsoft-Windows-Security-Auditing' Guid='{54849625-5478-4994-a5ba-3e3b0328c30d}'/><EventID>4625</EventID><Version>0</Version><Level>0</Level><Task>12544</Task><Opcode>0</Opcode><Keywords>0x8010000000000000</Keywords><TimeCreated SystemTime='2025-10-30T14:37:24.1621700Z'/><EventRecordID>1536271</EventRecordID><Correlation ActivityID='{2d231b4c-7851-0001-ff20-1ea65f45dc01}'/><Execution ProcessID='660' ThreadID='3060'/><Channel>Security</Channel><Computer>workstation-0</Computer><Security/></System><EventData><Data Name='SubjectUserSid'>S-1-0-0</Data><Data Name='SubjectUserName'>-</Data><Data Name='SubjectDomainName'>-</Data><Data Name='SubjectLogonId'>0x0</Data><Data Name='TargetUserSid'>S-1-0-0</Data><Data Name='TargetUserName'>ADMIN</Data><Data Name='TargetDomainName'>-</Data><Data Name='Status'>0xc000006d</Data><Data Name='FailureReason'>%%2313</Data><Data Name='SubStatus'>0xc0000064</Data><Data Name='LogonType'>3</Data><Data Name='LogonProcessName'>NtLmSsp </Data><Data Name='AuthenticationPackageName'>NTLM</Data><Data Name='WorkstationName'>-</Data><Data Name='TransmittedServices'>-</Data><Data Name='LmPackageName'>-</Data><Data Name='KeyLength'>0</Data><Data Name='ProcessId'>0x0</Data><Data Name='ProcessName'>-</Data><Data Name='IpAddress'>{{IP_ADDRESS}}</Data><Data Name='IpPort'>0</Data></EventData><RenderingInfo Culture='en-US'><Message>An account failed to log on.

Subject:
	Security ID:		S-1-0-0
	Account Name:		-
	Account Domain:		-
	Logon ID:		0x0

Logon Type:			3

Account For Which Logon Failed:
	Security ID:		S-1-0-0
	Account Name:		ADMIN
	Account Domain:		-

Failure Information:
	Failure Reason:		Unknown user name or bad password.
	Status:			0xC000006D
	Sub Status:		0xC0000064

Process Information:
	Caller Process ID:	0x0
	Caller Process Name:	-

Network Information:
	Workstation Name:	-
	Source Network Address:	{{IP_ADDRESS}}
	Source Port:		0

Detailed Authentication Information:
	Logon Process:		NtLmSsp 
	Authentication Package:	NTLM
	Transited Services:	-
	Package Name (NTLM only):	-
	Key Length:		0

This event is generated when a logon request fails. It is generated on the computer where access was attempted.

The Subject fields indicate the account on the local system which requested the logon. This is most commonly a service such as the Server service, or a local process such as Winlogon.exe or Services.exe.

The Logon Type field indicates the kind of logon that was requested. The most common types are 2 (interactive) and 3 (network).

The Process Information fields indicate which account and process on the system requested the logon.

The Network Information fields indicate where a remote logon request originated. Workstation name is not always available and may be left blank in some cases.

The authentication information fields provide detailed information about this specific logon request.
	- Transited services indicate which intermediate services have participated in this logon request.
	- Package name indicates which sub-protocol was used among the NTLM protocols.
	- Key length indicates the length of the generated session key. This will be 0 if no session key was requested.</Message><Level>Information</Level><Task>Logon</Task><Opcode>Info</Opcode><Channel>Security</Channel><Provider>Microsoft Windows security auditing.</Provider><Keywords><Keyword>Audit Failure</Keyword></Keywords></RenderingInfo></Event>
`)

// ServiceControlManagerTemplateName is the key for the Service Control Manager Windows Event template.
const ServiceControlManagerTemplateName = "service_control_manager"

// serviceControlManagerXMLTemplate is the Windows Event XML for Service Control Manager events.
var serviceControlManagerXMLTemplate = strings.TrimSpace(`
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='Service Control Manager' Guid='{555908d1-a6d7-4695-8e1e-26931d2012f4}' EventSourceName='Service Control Manager'/><EventID Qualifiers='16384'>7036</EventID><Version>0</Version><Level>4</Level><Task>0</Task><Opcode>0</Opcode><Keywords>0x8080000000000000</Keywords><TimeCreated SystemTime='2025-11-10T20:22:31.5396188Z'/><EventRecordID>3337</EventRecordID><Correlation/><Execution ProcessID='640' ThreadID='3836'/><Channel>System</Channel><Computer>iis-east1-prd-0</Computer><Security/></System><EventData><Data Name='param1'>Network Setup Service</Data><Data Name='param2'>stopped</Data><Binary>4E0065007400530065007400750070005300760063002F0031000000</Binary></EventData><RenderingInfo Culture='en-US'><Message>The Network Setup Service service entered the stopped state.</Message><Level>Information</Level><Task></Task><Opcode></Opcode><Channel></Channel><Provider>Microsoft-Windows-Service Control Manager</Provider><Keywords><Keyword>Classic</Keyword></Keywords></RenderingInfo></Event>
`)

// SuccessfulLogonTemplateName is the key for the successful logon Windows Event template.
const SuccessfulLogonTemplateName = "successful_logon"

// successfulLogonXMLTemplate is the Windows Event XML for successful logon events.
// It uses {{HOSTNAME}} to be replaced at render time. The hostname appears in both
// the SubjectUserName (with trailing $) and Computer (lowercase, no $) fields.
var successfulLogonXMLTemplate = strings.TrimSpace(`
<Event xmlns='http://schemas.microsoft.com/win/2004/08/events/event'><System><Provider Name='Microsoft-Windows-Security-Auditing' Guid='{54849625-5478-4994-a5ba-3e3b0328c30d}'/><EventID>4624</EventID><Version>2</Version><Level>0</Level><Task>12544</Task><Opcode>0</Opcode><Keywords>0x8020000000000000</Keywords><TimeCreated SystemTime='2025-11-10T20:19:31.1617052Z'/><EventRecordID>116138</EventRecordID><Correlation ActivityID='{49934b4b-ff0d-0001-6a93-d44c894cdc01}'/><Execution ProcessID='656' ThreadID='4792'/><Channel>Security</Channel><Computer>{{HOSTNAME_LOWER}}</Computer><Security/></System><EventData><Data Name='SubjectUserSid'>S-1-5-18</Data><Data Name='SubjectUserName'>{{HOSTNAME_UPPER}}$</Data><Data Name='SubjectDomainName'>WORKGROUP</Data><Data Name='SubjectLogonId'>0x3e7</Data><Data Name='TargetUserSid'>S-1-5-18</Data><Data Name='TargetUserName'>SYSTEM</Data><Data Name='TargetDomainName'>NT AUTHORITY</Data><Data Name='TargetLogonId'>0x3e7</Data><Data Name='LogonType'>5</Data><Data Name='LogonProcessName'>Advapi  </Data><Data Name='AuthenticationPackageName'>Negotiate</Data><Data Name='WorkstationName'>-</Data><Data Name='LogonGuid'>{00000000-0000-0000-0000-000000000000}</Data><Data Name='TransmittedServices'>-</Data><Data Name='LmPackageName'>-</Data><Data Name='KeyLength'>0</Data><Data Name='ProcessId'>0x280</Data><Data Name='ProcessName'>C:\Windows\System32\services.exe</Data><Data Name='IpAddress'>-</Data><Data Name='IpPort'>-</Data><Data Name='ImpersonationLevel'>%%1833</Data><Data Name='RestrictedAdminMode'>-</Data><Data Name='TargetOutboundUserName'>-</Data><Data Name='TargetOutboundDomainName'>-</Data><Data Name='VirtualAccount'>%%1843</Data><Data Name='TargetLinkedLogonId'>0x0</Data><Data Name='ElevatedToken'>%%1842</Data></EventData><RenderingInfo Culture='en-US'><Message>An account was successfully logged on.



Subject:

	Security ID:		S-1-5-18

	Account Name:		{{HOSTNAME_UPPER}}$

	Account Domain:		WORKGROUP

	Logon ID:		0x3E7

Logon Information:

	Logon Type:		5

	Restricted Admin Mode:	-

	Virtual Account:		No

	Elevated Token:		Yes

Impersonation Level:		Impersonation

New Logon:

	Security ID:		S-1-5-18

	Account Name:		SYSTEM

	Account Domain:		NT AUTHORITY

	Logon ID:		0x3E7

	Linked Logon ID:		0x0

	Network Account Name:	-

	Network Account Domain:	-

	Logon GUID:		{00000000-0000-0000-0000-000000000000}

Process Information:

	Process ID:		0x280

	Process Name:		C:\Windows\System32\services.exe

Network Information:

	Workstation Name:	-

	Source Network Address:	-

	Source Port:		-

Detailed Authentication Information:

	Logon Process:		Advapi  

	Authentication Package:	Negotiate

	Transited Services:	-

	Package Name (NTLM only):	-

	Key Length:		0

This event is generated when a logon session is created. It is generated on the computer that was accessed.

The subject fields indicate the account on the local system which requested the logon. This is most commonly a service such as the Server service, or a local process such as Winlogon.exe or Services.exe.

The logon type field indicates the kind of logon that occurred. The most common types are 2 (interactive) and 3 (network).

The New Logon fields indicate the account for whom the new logon was created, i.e. the account that was logged on.

The network fields indicate where a remote logon request originated. Workstation name is not always available and may be left blank in some cases.

The impersonation level field indicates the extent to which a process in the logon session can impersonate.

The authentication information fields provide detailed information about this specific logon request.

	- Logon GUID is a unique identifier that can be used to correlate this event with a KDC event.

	- Transited services indicate which intermediate services have participated in this logon request.

	- Package name indicates which sub-protocol was used among the NTLM protocols.

	- Key length indicates the length of the generated session key. This will be 0 if no session key was requested.</Message><Level>Information</Level><Task>Logon</Task><Opcode>Info</Opcode><Channel>Security</Channel><Provider>Microsoft Windows security auditing.</Provider><Keywords><Keyword>Audit Success</Keyword></Keywords></RenderingInfo></Event>
`)
