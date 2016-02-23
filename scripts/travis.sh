#!/bin/bash -e

apt-get update -qq
apt-get install -y libssl-dev openssl libssh2-1-dev wget fuse libfuse2 libfuse-dev
wget -O /tmp/v0.23.4.tar.gz https://github.com/libgit2/libgit2/archive/v0.23.4.tar.gz
tar xvf /tmp/v0.23.4.tar.gz -C /tmp
mkdir /tmp/libgit2-0.23.4/build
cd /tmp/libgit2-0.23.4/build
cmake -DTHREADSAFE=ON -DBUILD_CLAR=OFF -DCMAKE_BUILD_TYPE="RelWithDebInfo" ..
make
make install
ldconfig
