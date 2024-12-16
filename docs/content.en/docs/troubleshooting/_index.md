---
title: "FAQs"
weight: 50
---

# FAQs and Troubleshooting

FAQs about INFINI Framework and handling methods are provided here. You are welcome to submit your problems [here](https://github.com/infinilabs/framework/issues/new).

## FAQs

### Port Reuse Is Not Supported

Error prompt: The OS doesn't support SO_REUSEPORT: cannot enable SO_REUSEPORT: protocol not available

Fault description: Port reuse is enabled on INFINI Framework by default. It is used for multi-process port sharing. Patches need to be installed in the Linux kernel of the old version so that the port reuse becomes available.

Solution: Modify the network monitoring configuration by changing `reuse_port` to `false` to disable port reuse.

```
**.
   network:
     binding: 0.0.0.0:xx
     reuse_port: false
```