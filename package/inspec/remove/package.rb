describe package('blitz') do
    it { should_not be_installed }
end

describe file('/usr/bin/blitz') do
    it { should_not exist }
end

# Uninstall should not remove user.
describe user('blitz') do
    it { should exist }
end

# Uninstall should ot remove group.
describe group('blitz') do
    it { should exist }
end

describe systemd_service('blitz') do
    it { should_not be_installed }
    it { should_not be_enabled }
    it { should_not be_running }
end

describe processes('blitz') do
    it { should_not exist }
end