# syntax=docker/dockerfile:1@sha256:4edf897a3ffa55b89f906fc8cc78afdb3f1834cc9c7083565e611a8a7d5fe99e
# renovate: datasource=docker depName=quay.io/fedora/fedora-bootc
ARG BASE_IMAGE=quay.io/fedora/fedora-bootc:44@sha256:38e6bbf7e1127d775482a62763c25d0f00fca392bb88176c2ae265ad8ccbc525

# renovate: datasource=docker depName=golang
ARG GOLANG_IMAGE=golang:1.27@sha256:e432b43af23a9328d56a7c499be0476810aa344acbcf65fc7c455d4ff5a40602
# Raspberry Pi OpenOCD fork, `git describe` sdk-2.2.0-2-gcd4873400 (ADR 0007).
# renovate: datasource=git-refs depName=https://github.com/raspberrypi/openocd
ARG OPENOCD_REF=acff23ffd100479d6481a1155123774b79a990b9
# renovate: datasource=git-refs depName=https://github.com/dj-on-github/korad_control
ARG KORAD_CONTROL_REF=ed806d8
# Runner seed tarball, x86_64 only for now (ADR 0004). The sha256 is from the
# release notes and verified again at build time.
# renovate: datasource=github-releases depName=actions/runner
ARG RUNNER_VERSION=2.338.0
ARG RUNNER_SHA256=af4b794c1bc41d73d40535e3fe092a39f9679cd8d965954c2aca25a05ca41d32

# Go toolchain lives only here; the final image gets the static binaries.
FROM ${GOLANG_IMAGE} AS gobuild
WORKDIR /src
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -trimpath -o /out/ ./cmd/proves-runner ./cmd/proves-update

# Build-only packages stay in this stage (ADR 0007).
FROM ${BASE_IMAGE} AS hil
ARG OPENOCD_REF
ARG KORAD_CONTROL_REF
RUN dnf -y install --setopt=install_weak_deps=False \
      git gcc make autoconf automake libtool pkgconf-pkg-config texinfo \
      libusb1-devel hidapi-devel jimtcl-devel python3.13 \
    && dnf clean all
WORKDIR /build/openocd
RUN git clone https://github.com/raspberrypi/openocd . \
    && git checkout "${OPENOCD_REF}" \
    && ./bootstrap \
    && ./configure --prefix=/usr/local --enable-cmsis-dap --disable-werror \
    && make -j"$(nproc)" \
    && make install DESTDIR=/out \
    && install -Dm644 contrib/60-openocd.rules /out/usr/lib/udev/rules.d/60-openocd.rules
# The venv is built at its final path; its interpreter is the same python3.13
# RPM the final image installs. Workflows run ~/korad_control/.venv/bin/python.
RUN git clone https://github.com/dj-on-github/korad_control /usr/local/lib/korad_control \
    && git -C /usr/local/lib/korad_control checkout "${KORAD_CONTROL_REF}" \
    && rm -rf /usr/local/lib/korad_control/.git \
    && python3.13 -m venv /usr/local/lib/korad_control/.venv \
    && /usr/local/lib/korad_control/.venv/bin/pip install --no-cache-dir \
         pyserial==3.5 pyyaml==6.0.3 future==1.0.0 iso8601==2.1.0 \
    && /usr/local/lib/korad_control/.venv/bin/python /usr/local/lib/korad_control/korad_control.py --help

# The runner seed: downloaded and verified here, copied into the final image.
FROM ${BASE_IMAGE} AS seed
ARG RUNNER_VERSION
ARG RUNNER_SHA256
RUN curl -fsSL --retry 3 --max-time 600 -o /runner.tar.gz \
      "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz" \
    && echo "${RUNNER_SHA256}  /runner.tar.gz" > /runner.sha256 \
    && sha256sum -c /runner.sha256

FROM ${BASE_IMAGE}

# Tailscale is not in Fedora; use its stable repo (ADR 0005).
RUN curl -fsSL --retry 3 --max-time 60 \
      -o /etc/yum.repos.d/tailscale.repo \
      https://pkgs.tailscale.com/stable/fedora/tailscale.repo

# Runtime packages only; build-only packages belong in the builder stage
# (ADR 0007). No Wi-Fi packages: CI machines are wired (CONTEXT.md).
# Runner .NET deps: lttng-ust openssl-libs krb5-libs zlib libicu, plus ldd
# (glibc-common), which config.sh checks for.
RUN dnf -y install --setopt=install_weak_deps=False \
      git make cmake gcc gcc-c++ curl psmisc lsof procps-ng usbutils jq screen \
      tailscale NetworkManager \
      lttng-ust openssl-libs krb5-libs zlib-ng-compat libicu glibc-common \
      libusb1 hidapi jimtcl python3.13 \
    && test -z "$(rpm -qa ModemManager NetworkManager-wifi wpa_supplicant)" \
    && dnf clean all \
    && rm -rf /var/cache/dnf /var/lib/dnf/repos

COPY files/ /
COPY --from=gobuild /out/ /usr/bin/
COPY --from=seed /runner.tar.gz /usr/lib/proves-ci-os/actions-runner.tar.gz
COPY --from=hil /out/usr/ /usr/
COPY --from=hil /usr/local/lib/korad_control/ /usr/local/lib/korad_control/

# ops and runner come from sysusers.d (ADR 0005); run it now so the accounts
# exist in the built image, not just after first boot. dialout lives in
# /usr/lib/group, where sysusers `m` cannot add members, so copy it to /etc/group and usermod.
RUN systemd-sysusers \
    && grep '^dialout:' /usr/lib/group >> /etc/group \
    && usermod -aG dialout ops \
    && systemctl disable sshd.service bootc-fetch-apply-updates.timer \
    && systemctl enable proves-update.timer actions-runner-seed.service actions-runner.service \
    && ln -sf /usr/share/zoneinfo/UTC /etc/localtime \
    && echo 'LANG=en_US.UTF-8' > /etc/locale.conf

RUN id ops runner
