# Partial kickstart (ADR 0008). Anaconda prompts for every required item
# omitted here: the ops password, the hostname and the target disk.
# The payload comes from the ISO's own container storage, so install is offline.
# `text` here, not just inst.text on the kernel line: only a kickstart `text`
# stops Anaconda asking whether to use RDP instead (it still prompts for the rest).
text
bootc --source-imgref containers-storage:ghcr.io/open-source-space-foundation/proves-ci-os:latest --target-imgref ghcr.io/open-source-space-foundation/proves-ci-os:latest
network --bootproto=dhcp --device=link --activate --onboot=on
timezone --utc UTC
lang en_US.UTF-8
keyboard us
rootpw --lock
# The Image already defines ops (sysusers.d, ADR 0005). Naming it here keeps Anaconda
# from asking for a generic user's name; it should ask only for the password.
user --name=ops --gecos="Shared ops account" --homedir=/var/home/ops --groups=wheel
clearpart --all --initlabel
reboot --eject
