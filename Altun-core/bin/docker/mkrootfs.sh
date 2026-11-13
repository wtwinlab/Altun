#!/usr/bin/env bash
# Build the root file system of the Docker/WAN emulation image from the host's own
# (Ubuntu 20.04) binaries: no network access, no base image pulled. Output: rootfs.tar,
# imported with
#   docker import rootfs.tar altunwan_base:v1          (ALTUNWAN_IMAGE in bin/env.sh)
# Contents: bash/dash, a few coreutils/procps tools, iproute2 (ip, tc, ss + tc dist tables),
# ping, and the static Go probe `altunwan-probe` (built by bin/build.sh). The protocol
# binaries are not in the image: they are bind-mounted from REMOTE_DIR and ALTUNWAN_DIR,
# whose mount points are created here.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
. "$HERE/../env.sh"
RF="$HERE/rootfs"
mkdir -p "$RF"/{usr/bin,usr/sbin,usr/lib/x86_64-linux-gnu,usr/lib64,usr/lib/tc,etc,tmp,proc,sys,dev,root,run,var/tmp} "$RF$REMOTE_DIR" "$RF$ALTUNWAN_DIR"
chmod 1777 "$RF/tmp" "$RF/var/tmp"
for l in bin sbin lib lib64; do [ -e "$RF/$l" ] || ln -s usr/$l "$RF/$l"; done
BINS="/bin/bash /bin/dash /bin/cat /bin/sleep /usr/bin/env /bin/ls /bin/cp /bin/mkdir /bin/date /bin/echo /bin/true /bin/false
/usr/bin/id /usr/bin/tail /usr/bin/head /bin/grep /bin/sed /usr/bin/wc /usr/bin/awk /usr/bin/mawk /bin/ps /usr/bin/pgrep /usr/bin/pkill /usr/bin/timeout /usr/bin/tee /usr/bin/nproc
/sbin/ip /sbin/tc /bin/ss /bin/ping"
for b in $BINS; do
  rb=$(readlink -f "$b"); cp -L "$rb" "$RF/usr/bin/" 2>/dev/null || true
  case "$b" in /sbin/*) cp -L "$rb" "$RF/usr/sbin/";; esac
  for lib in $(env -u LD_LIBRARY_PATH ldd "$rb" | awk '/=>/ {print $3} /ld-linux/ {print $1}' | grep '^/'); do
    d="$RF$(dirname "$(readlink -f "$lib")")"; mkdir -p "$d"
    cp -L "$lib" "$RF$lib" 2>/dev/null || { mkdir -p "$RF$(dirname "$lib")"; cp -L "$lib" "$RF$lib"; }
  done
done
ln -sf dash "$RF/usr/bin/sh"; ln -sf gawk "$RF/usr/bin/awk"
cp /usr/lib/tc/*.dist "$RF/usr/lib/tc/"
cp "$HERE/../altunwan-probe" "$RF/usr/bin/"
printf 'root:x:0:0:root:/root:/bin/bash\n%s:x:%s:%s::/tmp:/bin/bash\n' "$(id -un)" "$(id -u)" "$(id -g)" > "$RF/etc/passwd"
printf 'root:x:0:\n%s:x:%s:\n' "$(id -gn)" "$(id -g)" > "$RF/etc/group"
echo "hosts: files dns" > "$RF/etc/nsswitch.conf"
(cd "$RF" && tar --numeric-owner --owner=0 --group=0 -cf "$HERE/rootfs.tar" .)
ls -la "$HERE/rootfs.tar"
