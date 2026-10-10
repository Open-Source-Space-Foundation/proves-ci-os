# Partial kickstart (ADR 0008). Anaconda prompts for every required item
# omitted here: the ops password, the hostname and the target disk.
# The payload comes from the ISO's own container storage, so install is offline.
bootc --source-imgref containers-storage:ghcr.io/open-source-space-foundation/proves-ci-os:latest --target-imgref ghcr.io/open-source-space-foundation/proves-ci-os:latest
network --bootproto=dhcp --device=link --activate --onboot=on
timezone --utc UTC
lang en_US.UTF-8
keyboard us
rootpw --lock
clearpart --all --initlabel
reboot --eject
