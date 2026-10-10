# syntax=docker/dockerfile:1
# renovate: datasource=docker depName=quay.io/fedora/fedora-bootc
ARG BASE_IMAGE=quay.io/fedora/fedora-bootc:44@sha256:c24cce42b92975040c0a725c7eef9d6c3bd6f3de4165d947dcd4dff4ab57c3ab

FROM ${BASE_IMAGE}

# Tailscale is not in Fedora; use its stable repo (ADR 0005).
RUN curl -fsSL --retry 3 --max-time 60 \
      -o /etc/yum.repos.d/tailscale.repo \
      https://pkgs.tailscale.com/stable/fedora/tailscale.repo

# Runtime packages only; build-only packages belong in the builder stage
# (ADR 0007). No Wi-Fi packages: CI machines are wired (CONTEXT.md).
# Runner .NET deps: lttng-ust openssl-libs krb5-libs zlib libicu.
RUN dnf -y install --setopt=install_weak_deps=False \
      git make cmake gcc gcc-c++ curl psmisc lsof procps-ng usbutils jq screen \
      tailscale NetworkManager \
      lttng-ust openssl-libs krb5-libs zlib-ng-compat libicu \
      libusb1 hidapi python3.13 \
    && test -z "$(rpm -qa ModemManager NetworkManager-wifi wpa_supplicant)" \
    && dnf clean all \
    && rm -rf /var/cache/dnf /var/lib/dnf/repos

COPY files/ /

# ops and runner come from sysusers.d (ADR 0005); run it now so the accounts
# exist in the built image, not just after first boot. dialout lives in
# /usr/lib/group, where sysusers `m` cannot add members, so copy it to /etc/group and usermod.
RUN systemd-sysusers \
    && grep '^dialout:' /usr/lib/group >> /etc/group \
    && usermod -aG dialout ops \
    && systemctl disable sshd.service \
    && ln -sf /usr/share/zoneinfo/UTC /etc/localtime \
    && echo 'LANG=en_US.UTF-8' > /etc/locale.conf

RUN id ops runner
