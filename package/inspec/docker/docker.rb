describe user('bploader') do
    it { should exist }
    its('uid') { should eq 10001 }
    its('group') { should eq 'bploader' }
    its('lastlogin') { should eq nil }
end

describe file('/bindplane-loader') do
    its('mode') { should cmp '0755' }
    its('owner') { should eq 'root' }
    its('group') { should eq 'root' }
    its('type') { should cmp 'file' }
end