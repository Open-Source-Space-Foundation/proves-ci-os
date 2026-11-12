# Partial kickstart (ADR 0008). Anaconda prompts for every required item
# omitted here: the ops password (asked for as root's), the hostname and the target disk.
# The payload comes from the ISO's own container storage, so install is offline.
# `text` here, not just inst.text on the kernel line: only a kickstart `text`
# stops Anaconda asking whether to use RDP instead (it still prompts for the rest).
text
bootc --source-imgref containers-storage:ghcr.io/open-source-space-foundation/proves-ci-os:latest --target-imgref ghcr.io/open-source-space-foundation/proves-ci-os:latest
network --bootproto=dhcp --device=link --activate --onboot=on
timezone --utc UTC
lang en_US.UTF-8
keyboard us
# No rootpw line: Anaconda then asks for a root password, and %post below gives it to ops.
# `user --name=ops` cannot do this. The Image already defines ops (sysusers.d, ADR 0005),
# so Anaconda fails with "User ops already exists" and never sets the password.
clearpart --all --initlabel
reboot --eject

# Move the password entered for root onto ops, then lock root again. Fail the
# install rather than leave ops without a usable password.
%post --erroronfail
set -eu
hash="$(getent shadow root | cut -d: -f2)"
case "$hash" in
  ''|'!'*|'*'*) echo "no root password was set; cannot give ops a password" >&2; exit 1 ;;
esac
usermod --password "$hash" ops
passwd --lock root
%end
