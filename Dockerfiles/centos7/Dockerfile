# Copyright 2020 VMware, Inc.
#
# SPDX-License-Identifier: Apache-2.0

FROM centos:7

# RUN sed -i -e '/mirrors.cloud.aliyuncs.com/d' -e '/mirrors.aliyuncs.com/d' /etc/yum.repos.d/CentOS-Base.repo
RUN yum update -y && yum install -y open-vm-tools \
	    build-essential \
        ca-certificates \
        curl \
	    kmod

# Start of CUDA base
ENV CUDA_VERSION 10.0.130
ENV CUDA_PKG_VERSION 10-0-$CUDA_VERSION
# For libraries in the cuda-compat-* package: https://docs.nvidia.com/cuda/eula/index.html#attachment-a

RUN yum-config-manager --add-repo http://developer.download.nvidia.com/compute/cuda/repos/rhel7/x86_64/cuda-rhel7.repo
RUN	yum install -y -q cuda-cudart-$CUDA_PKG_VERSION \
        cuda-compat-10-0 && \
    ln -s cuda-10.0 /usr/local/cuda

ENV PATH /usr/local/nvidia/bin:/usr/local/cuda/bin:${PATH}
ENV LD_LIBRARY_PATH /usr/local/nvidia/lib:/usr/local/nvidia/lib64

# Install CUDA runtime
ENV	NCCL_VERSION 2.5.6
RUN	yum install -y -q cuda-libraries-$CUDA_PKG_VERSION \
	    cuda-nvtx-$CUDA_PKG_VERSION 
RUN	yum install -y -q https://developer.download.nvidia.com/compute/machine-learning/repos/rhel7/x86_64/nvidia-machine-learning-repo-rhel7-1.0.0-1.x86_64.rpm

# yum clean all
RUN	yum update -q && \
	yum install -y -q libnccl-$NCCL_VERSION-1+cuda10.0 && \
	echo exclude=libnccl >> /etc/yum.conf

# Install CUDA cudnn
ENV	CUDNN_VERSION 7.6.5.32

RUN yum install -y -q libcudnn7-$CUDNN_VERSION-1.cuda10.0 && \
	echo exclude=libcudnn7 >> /etc/yum.conf

# Start of bitfusion
RUN yum install -y -q wget json-c-devel librdmacm libibverbs libuuid \
        proc-ng-devel && \
    rm -rf /var/lib/apt/lists/*

# TODO: Make this work with Bitfusion GA version
RUN yum install -y epel-release 
RUN rpm --import https://packages.vmware.com/bitfusion/vmware.bitfusion.key

RUN cd /tmp && \
    curl -fSslL -O https://packages.vmware.com/bitfusion/centos/7/bitfusion-client-centos7-3.0.0-11.x86_64.rpm && \
    yum install -y ./bitfusion-client-centos7-3.0.0-11.x86_64.rpm && \
    rm -rf /tmp/*

