describe package('blitz') do
    it { should be_installed }
end

describe file('/usr/bin/blitz') do
    its('mode') { should cmp '0755' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end

describe file('/etc/blitz/config.yaml') do
    its('mode') { should cmp '0640' }
    its('owner') { should eq 'blitz' }
    its('group') { should eq 'blitz' }
    its('type') { should cmp 'file' }
end
 
describe file('/etc/blitz/blitz.env') do
    its('mode') { should cmp '0640' }
    its('owner') { should eq 'blitz' }
    its('group') { should eq 'blitz' }
    its('type') { should cmp 'file' }
end

%w[LICENSE configuration.md metrics.md].each do |doc_file|
describe file("/usr/share/doc/blitz/#{doc_file}") do
    its('mode') { should cmp '0644' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end
end

describe file('/usr/lib/systemd/system/blitz.service') do
    its('mode') { should cmp '0640' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end

describe user('blitz') do
    it { should exist }
    its('group') { should eq 'blitz' }
    its('lastlogin') { should eq nil }
    its('shell') { should eq '/sbin/nologin' }
end

describe group('blitz') do
    it { should exist }
end

describe systemd_service('blitz') do
    it { should be_installed }
    it { should_not be_enabled }
    it { should_not be_running }
end

describe directory('/var/log/blitz') do
    it { should exist }
    its('mode') { should cmp '0755' }
    its('owner') { should eq 'blitz' }
    its('group') { should eq 'blitz' }
end

describe file('/var/log/blitz/blitz.log') do
    it { should exist }
    its('mode') { should cmp '0644' }
    its('owner') { should eq 'blitz' }
    its('group') { should eq 'blitz' }
    its('type') { should cmp 'file' }
end

describe json(command: 'blitz version') do
    its(['buildPlatform']) { should eq 'linux/amd64' }
    its(['binaryPlatform']) { should eq 'linux/amd64' }
end
