describe package('bindplane-loader') do
    it { should_not be_installed }
end

describe file('/usr/bin/bindplane-loader') do
    it { should_not exist }
end

# Uninstall should not remove user.
describe user('bploader') do
    it { should exist }
end

# Uninstall should ot remove group.
describe group('bploader') do
    it { should exist }
end

describe systemd_service('bindplane-loader') do
    it { should_not be_installed }
    it { should_not be_enabled }
    it { should_not be_running }
end

describe processes('bindplane-loader') do
    it { should_not exist }
end