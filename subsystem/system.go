package subsystem

type ResourceConfig struct {
	MemoryLimit string
	CPUShare    string
	CPUSet      string
}

type Subsystem interface {
	// 返回subsystem的名字
	// cpu memory cpuset
	Name() string
	// 设置某个cgroup在当前subsystem中的资源限制
	Set(path string, res *ResourceConfig) error
	// 将某个进程添加到当前subsystem下的cgroup中
	Apply(path string, pid int) error
	// 移除某个cgroup
	Remove(path string) error
}
