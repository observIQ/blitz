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

describe file('/usr/share/doc/blitz/LICENSE') do
    its('mode') { should cmp '0644' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
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
