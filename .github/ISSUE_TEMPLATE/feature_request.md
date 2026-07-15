---
name: Feature request
about: Suggest an improvement to the library, CLI, or docs
title: "[feature] "
labels: enhancement
assignees: ""
---

## Problem

<!-- What are you trying to do that kumokagi makes hard or impossible today?
Describe the situation, not just the feature. -->

## Proposed solution

<!-- What would you like kumokagi to do? A config field, a CLI flag, an API? -->

## Does it fit the design principles?

<!--
kumokagi deliberately: never persists secrets, never stores credentials (ambient
only), stays a *source* in viper/pydantic-settings rather than a config system,
and keeps rotation free by not caching. See the ADRs:
https://stlimtat.github.io/kumokagi/adrs/

If your request touches one of these, note how it fits (or why the decision
should change).
-->

## Alternatives considered

<!-- Workarounds you've tried, other tools, or why existing features don't cover it. -->
