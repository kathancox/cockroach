// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import classNames from "classnames/bind";
import React from "react";

import Back from "./back-arrow.svg";
import styles from "./backIcon.module.scss";

const cx = classNames.bind(styles);

export const BackIcon = (): React.ReactElement => (
  <img src={Back} alt="back" className={cx("root")} />
);
