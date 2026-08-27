# Seismic Event Associator

纯Go地震波形检测与震相关联服务。系统流式解析miniSEED2子集，对每个通道执行去均值、去趋势、IIR带通和STA/LTA检测，输出带证据的P/S拾取，再使用台站坐标、可替换走时模型和空间网格求解事件起时与震源位置。

## 已实现能力

- miniSEED2固定头、BTIME、采样率因子/乘数、时间修正和Blockette 1000链解析。
- 校验记录长度指数、数据偏移、字节序、样本数量和有限浮点值；支持Int16、Int32、Float32和Float64编码。
- Steim 1/2通过`SteimDecoder`接口替换，未配置时明确拒绝，不伪装已解码。
- 多记录按台站/通道/起时排序，检测重复摘要、重叠、采样率变化和时间缺口。
- 去均值、线性去趋势、多阶IIR带通、统计量、STA/LTA和基于中位数/MAD的噪声自适应阈值。
- 垂直通道生成P候选，水平通道生成S候选；拾取保留时间不确定度、振幅、SNR、触发比和输入记录摘要。
- 台站元数据按版本和有效期管理，坐标、经度、纬度与单调版本均校验。
- 均匀速度走时模型实现球面距离、台站高程与震源深度，P/S速度独立且可由接口替换。
- 网格搜索通过各拾取反推起时中位数，以时间残差、台站数和方位覆盖评分；返回RMS残差、方位缺口和置信度。
- 事件支持增量合并、操作员合并、按残差簇拆分、撤销、版本递增和supersedes关系。
- 震级按P波峰值、震中距和稳健中位数估算，输出逐台站校正项与MAD不确定度。
- 有界任务模型支持优先级、领取租约、超时重领、重试上限、取消、进度和结果引用。
- 证据索引记录输入摘要、参数、算法版本、拾取窗口、关联结果和震级观察量。

这不是通用告警平台。默认内存适配器用于本地可重复验收；PostgreSQL迁移定义台站版本、任务、拾取、事件版本和证据索引，波形正文通过`ObjectStore`接口独立保存。未接线外部存储时不宣称重启后保留状态。

## 数据确定性

- 乱序记录先按通道和起时排序。
- 同一输入流中摘要重复会返回`duplicate waveform record`。
- 小缺口可显式填充NaN，大缺口拆分为独立流；带缺口的单块进入拾取器时明确拒绝。
- BTIME时间修正仅在“修正尚未应用”标志下应用，避免重复校时。
- 任意非有限样本、错误字节序、越界Blockette、截断载荷和未配置Steim编码均拒绝。
- HTTP导入按记录顺序处理；每个成功记录的拾取独立持久，后续记录失败不会回滚已完成记录，调用方可用记录摘要幂等去重。
- 网格节点数受`max_nodes`限制，context取消会终止走时计算，不能发布部分关联结论。
- 慢证据消费者与波形正文存储位于热路径接口之后，生产适配器必须使用有界队列。

## 目录

```text
cmd/server                 HTTP服务和优雅停机
cmd/wave-simulator         合成miniSEED2四台站震相模拟器
internal/waveform          波形、流组装和对象存储接口
internal/seed              miniSEED头、编码器、解析器和解码器
internal/station           版本化台站目录
internal/signal            信号预处理、滤波、STA/LTA
internal/picker            P/S拾取、噪声门限和拾取仓储
internal/traveltime        球面几何与走时模型
internal/association       网格定位与事件生命周期
internal/magnitude         震级与逐台站观测
internal/job               优先任务、租约、取消和重试
internal/evidence          可复现证据索引
internal/platform          配置、中间件、指标和运行时装配
api/configs/migrations     REST/gRPC契约、配置和数据库迁移
deploy/scripts             容器与端到端冒烟交付
```

业务包均按`domain`、`application`、`adapter`、`infrastructure`边界预留；当前HTTP适配器集中在platform，内存仓储位于各域infrastructure中。

## 启动与验证

要求Go 1.22或更高版本。

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
go run ./cmd/server -config configs/local.yaml
```

默认监听`18336`。健康、就绪和指标无需认证，其余端点要求`X-API-Key: seismic-dev`。环境变量`SEISMIC_ADDRESS`、`SEISMIC_API_KEY`、`SEISMIC_NODE_ID`可覆盖YAML。

另一个终端运行：

```bash
go run ./cmd/wave-simulator -url http://127.0.0.1:18336
```

模拟器不会提交固定拾取或事件。它为四个台站计算真实P波走时，在对应样本处叠加脉冲，编码成8192字节miniSEED2 Float32记录，逐个调用导入接口，然后提交网格关联。正常输出包含每台站走时与拾取数，以及计算出的事件、残差、置信度和震级。

完整冒烟：

```bash
./scripts/smoke.sh
```

## API流程

查看内置台站：

```bash
curl -H 'X-API-Key: seismic-dev' http://127.0.0.1:18336/v1/stations
```

`POST /v1/waveforms`接受`miniseed_base64`，也接受便于诊断的JSON blocks。信号参数中的duration使用纳秒整数，例如STA 500ms为`500000000`。默认参数为0.5-12Hz、二阶带通、0.5s STA、5s LTA、2.0/1.2开关阈值。

关联请求示例：

```json
{
  "from": "2026-08-22T08:00:00Z",
  "to": "2026-08-22T08:02:00Z",
  "grid": {
    "min_latitude": -0.2,
    "max_latitude": 0.2,
    "min_longitude": -0.2,
    "max_longitude": 0.2,
    "horizontal_step": 0.1,
    "depths_km": [0, 5, 10],
    "max_nodes": 1000
  },
  "min_stations": 4,
  "max_residual_ms": 1000
}
```

结果查询：

```bash
curl -H 'X-API-Key: seismic-dev' http://127.0.0.1:18336/v1/picks
curl -H 'X-API-Key: seismic-dev' http://127.0.0.1:18336/v1/events
curl -H 'X-API-Key: seismic-dev' http://127.0.0.1:18336/v1/events/EVENT_ID
curl -H 'X-API-Key: seismic-dev' http://127.0.0.1:18336/v1/evidence/EVENT_ID
```

撤销事件要求原因并产生新版本：

```bash
curl -H 'X-API-Key: seismic-dev' -H 'Content-Type: application/json' \
  -d '{"reason":"station timing correction invalidated solution"}' \
  http://127.0.0.1:18336/v1/events/EVENT_ID/revoke
```

## 容器

```bash
docker compose -f deploy/docker-compose.yaml up --build
docker compose -f deploy/docker-compose.yaml down -v
```

Compose提供PostgreSQL供持久化适配器使用。当前可执行程序明确装配内存仓储，数据库迁移不会被假装成已接线能力。

## 验收重点

1. `/healthz`、`/readyz`为200，`/metrics`记录导入、拾取和事件数。
2. 合法miniSEED记录产生数据驱动拾取；截断或非法记录返回400。
3. 四个一致台站拾取形成事件，位置来自网格搜索，RMS来自真实预测残差。
4. 少于`min_stations`、超出残差或超出网格预算不能发布事件。
5. 事件、拾取和证据可分别查询，证据包含参数、摘要和算法版本。
6. SIGINT/SIGTERM触发优雅停机，停止后18336不再监听。

