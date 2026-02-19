# SPDX-FileCopyrightText: Copyright 2025 Stacklok, Inc.
# SPDX-License-Identifier: Apache-2.0

FROM alpine:3.21

RUN apk add --no-cache python3 py3-pip coreutils findutils \
    && python3 -m venv /opt/waggle-venv \
    && /opt/waggle-venv/bin/pip install --no-cache-dir --upgrade pip

ENV PATH="/opt/waggle-venv/bin:$PATH"
