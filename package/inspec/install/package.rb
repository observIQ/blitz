describe package('bindplane-loader') do
    it { should be_installed }
end

describe file('/usr/bin/bindplane-loader') do
    its('mode') { should cmp '0755' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end

describe file('/etc/bindplane-loader/config.yaml') do
    its('mode') { should cmp '0640' }
    its('owner') { should eq 'bploader' }
    its('group') { should eq 'bploader' }
    its('type') { should cmp 'file' }
end

describe file('/usr/share/doc/bindplane-loader/LICENSE') do
    its('mode') { should cmp '0644' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end

describe file('/usr/lib/systemd/system/bindplane-loader.service') do
    its('mode') { should cmp '0640' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end

describe user('bploader') do
    it { should exist }
    its('group') { should eq 'bploader' }
    its('lastlogin') { should eq nil }
    its('shell') { should eq '/sbin/nologin' }
end

describe group('bploader') do
    it { should exist }
end

describe systemd_service('bindplane-loader') do
    it { should be_installed }
    it { should_not be_enabled }
    it { should be_running }
end