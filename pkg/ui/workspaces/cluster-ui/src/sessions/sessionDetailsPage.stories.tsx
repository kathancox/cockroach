// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import { storiesOf } from "@storybook/react";
import React from "react";

import { withBackground, withRouterProvider } from "src/storybook/decorators";

import { SessionDetails } from "./sessionDetails";
import {
  sessionDetailsActiveStmtPropsFixture,
  sessionDetailsActiveTxnPropsFixture,
  sessionDetailsClosedPropsFixture,
  sessionDetailsIdlePropsFixture,
  sessionDetailsNotFound,
} from "./sessionDetailsPage.fixture";

storiesOf("Session Details Page", module)
  .addDecorator(withRouterProvider)
  .addDecorator(withBackground)
  .add("Idle Session", () => (
    <SessionDetails {...sessionDetailsIdlePropsFixture} />
  ))
  .add("Idle Txn", () => (
    <SessionDetails {...sessionDetailsActiveTxnPropsFixture} />
  ))
  .add("Session", () => (
    <SessionDetails {...sessionDetailsActiveStmtPropsFixture} />
  ))
  .add("Closed Session", () => (
    <SessionDetails {...sessionDetailsClosedPropsFixture} />
  ))
  .add("Session Not Found", () => (
    <SessionDetails {...sessionDetailsNotFound} />
  ));
