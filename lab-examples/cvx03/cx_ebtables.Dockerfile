# docker build \
# --force-rm=true \
# -t cx_ebtables:5.3.0 \
# -f cx_ebtables.Dockerfile .

# Fix for ebatble issue: https://github.com/networkop/cx/issues/13

from networkop/cx:5.3.0

RUN rm /sbin/ebtables
RUN ln -s /usr/sbin/ebtables /sbin/ebtables
