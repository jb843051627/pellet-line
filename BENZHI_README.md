基于 Go 实现的 pellet-line Web 项目，一款后端服务，完成生物质颗粒厂生产运营数据的采集、处理与报告管理。

内置原料进场批次化验、制粒批次质检、在线含水率抽检、设备巡检与保养计划等核心业务，使用 SQLite 磁盘数据库持久化，提供 JSON 接口供生产看板调用。

## Docker

```bash
./build_benzhi_docker.sh pellet-line linux/amd64
./build_benzhi_docker.sh pellet-line linux/arm64
```
