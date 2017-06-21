# build-your-own-docker
自己动手写Docker。 重复造轮子，初步定位是可以写一个能够类似于runc 一样的容器运行引擎，然后加上资源的隔离。最好能有image的支持，希望做出一个最简版的docker，帮助自己和感兴趣的同学深入理解 docker 的原理以及具有动手实践案例。

## 目录
- 前言
	
- 第一章 容器与开发语言
	- Docker
	- Golang

- 第二章 基础技术
	- Linux Namespace
  		- 概念
  		- UTS Namespace
  		- IPC Namespace
  		- PID Namespace
  		- Mount Namespace
  		- User Namespace
  		- Network Namespace		  
	- Linux Cgroups
  		- 什么是Linux Cgroups
  		- Docker是如何使用Cgroups的
  		- 用go语言实现通过cgroup限制容器的资源

  	- Union File System
  		- 什么是Union File System
  		- Docker是如何使用Union File System的
  		- 自己动手写Union File System 例子

- 第三章  构造容器
	-  构造实现run命令版本的容器
		- Linux proc 文件系统介绍
		- 实现 run 命令
	- 使用Cgroups 限制容器资源使用
		- 定义Cgroups的数据结构
		- 在启动容器的时候增加资源限制的配置 
	- 增加管道以及环境变量识别
		- 管道
		- PATH识别	
		
- 第四章 构造镜像 
	- 使用busybox创建容器
		- busybox
		- pivot_root
	- 使用 AUFS 包装busybox
	- 实现volume数据卷
	- 实现简单镜像打包
	
- 第五章 构建容器进阶
	- 实现容器的后台运行
	- 实现查看运行中容器
	- 实现查看容器日志
	- 实现进入容器Namespace
	- 实现停止容器
	- 实现删除容器
	- 实现通过容器制作镜像
	- 实现容器指定环境变量运行

- 第六章 容器网络
	- 容器虚拟化网络基础技术介绍
	- 构建容器网络模型
	- 容器地址分配
	- 创建Bridge网络
 	- 在Bridge网络创建容器
 	- 容器跨主机网络

- 第七章 高级实践	
	- 使用mydocker创建一个可访问nginx容器
	- 使用mydocker 创建一个flask + redis的计数器
	- runC介绍
	- runC创建容器流程
	- containerd介绍
	- kunernets CRI 容器引擎
 	
## 作者列表
- 陈显鹭 (阿里云容器服务团队)
- 王炳燊 (阿里云容器服务团队)
- 秦妤嘉 (阿里云容器服务团队)
