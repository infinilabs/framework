---
weight: 20
title: "Setup IntelliJ IDEA"
---

# Setup IntelliJ IDEA

This guide provides step-by-step instructions to configure IntelliJ IDEA for Go development, ensuring optimal compatibility with project requirements.

---

## Configure GOPATH

Set the `GOPATH` for your project in IntelliJ IDEA:

**Navigate to:**
`Preferences | Languages & Frameworks | Go | GOPATH`

![idea_settings.png](/img/references/idea_settings.png)

---

## Adjust Run/Debug Configurations

For projects that rely on legacy dependency management (e.g., the `vendor` folder), configure the necessary environment variables:

```bash
GO15VENDOREXPERIMENT="1"; GO111MODULE=off
```

Set these variables in:
Preferences | Run/Debug Configurations | Templates

## Add Copyright Header Templates

Ensure all new files include the appropriate copyright notice.

**Navigate to:**
`Preferences | Editor | File and Code Templates`
![idea_disable_modules.png](/img/references/idea_copyright_template.png)

Add the following header template:
```
/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */
```
## Disable Unused Modules (For Higher Versions)

In newer versions of IntelliJ IDEA, disabling unused modules can improve performance and reduce conflicts.
![idea_disable_modules.png](/img/references/idea_disable_modules.png)

To improve code quality, enable static analysis tools like GoLint or GoVet in IntelliJ IDEA.

## Optimize Performance
For large projects, increase IntelliJ IDEA’s memory allocation by editing the `idea.vmoptions` file.

Example:
```
-Xms1024m
-Xmx2048m
```
