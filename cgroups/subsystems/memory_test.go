package subsystems

import(
	"testing"
	"os"
	"path"
)

func TestMemoryCgroup(t *testing.T) {
	memSubSys := MemorySubSystem{}
	resConfig := ResourceConfig{
		MemoryLimit: "1000m",
	}
	testCgroup := "testmemlimit"

	if err := memSubSys.Set(testCgroup, &resConfig); err != nil {
		t.Fatalf("cgroup fail %v", err)
	}
	stat, _ := os.Stat(path.Join(FindCgroupMountpoint("memory"), testCgroup))
	t.Logf("cgroup stats: %+v", stat)

	if err := memSubSys.Apply(testCgroup, os.Getpid()); err != nil {
		t.Fatalf("cgroup Apply %v", err)
	}
	//将进程移回到根Cgroup节点
	if err := memSubSys.Apply("", os.Getpid()); err != nil {
		t.Fatalf("cgroup Apply %v", err)
	}

	if err := memSubSys.Remove(testCgroup); err != nil {
		t.Fatalf("cgroup remove %v", err)
	}
}
