//TODO
bytesbuffer 按照请求的类型，区分大中型，避免重复分配资源，如果已经存在大件的，取对应的 bytesbuffer 实例，bytesbuffer 限制数量个数，控制内存的使用情况。
分级别取 bytesbuffer 实例，别原地扩容。