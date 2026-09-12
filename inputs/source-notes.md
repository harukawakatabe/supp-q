# Make-PRD 输入材料

## 已确认产品合同

- 完整目标功能以 MVP 当前实现和 MVP PRD 为产品母版。
- MVP/Web 是视觉与交互基线；保留视觉语言，允许为触控、无障碍和真实状态做修正。
- Uni 是唯一正式工程底座；保留服务端权威数据、身份隔离、异步任务、私有存储、确定性规则、FEFO 与精确撤销。
- 当前只做补剂，不做处方药，也不提供假入口。
- AI 只做已确认补剂数据的机械汇总、资料解释和来源提示；不做剂量裁决、诊断或相互作用结论。
- H5 先做站内提醒与通知权限，外部通知作为后续目标。
- 可分阶段实现，但不得删除完整目标范围。

## 主要输入文件

| 优先级 | 路径 | 用途 |
| --- | --- | --- |
| 1 | `shaping/06-shaped-brief.md` | 用户确认后的定型摘要 |
| 2 | `shaping/01-product-shaping.md` | 定位、用户、价值和形态 |
| 3 | `shaping/03-scope.md` | 完整目标范围、非目标和指标 |
| 4 | `shaping/04-pages-and-flows.md` | 页面、主流程和跨页约束 |
| 5 | `shaping/07-feature-parity-matrix.md` | MVP/Web/Uni 逐功能差距 |
| 6 | `shaping/08-visual-baseline.md` | 视觉语言、组件和验收方向 |
| 7 | `shaping/09-rule-and-data-delta.md` | 领域规则、对象和单一事实来源 |
| 8 | `mvp/PRD.md` 及 MVP 实现 | 完整目标功能与旧规则证据，只读 |
| 9 | `uni/PRD.md`、`uni/docs/`、`uni/app/`、`uni/server/`、`uni/contracts/` | 当前正式工程事实，只读 |

## 写作约束

- 用 `CONFIRMED / CODE-VERIFIED / DOCUMENTED / DRAFT / OPEN / DEFERRED-ORDER` 区分证据状态。
- 不把旧 PRD 的目标指标写成已实现结果。
- 不把历史测试通过写成当前生产可用。
- MVP 代码和 PRD 冲突时不猜测，在对应模块提出最少量的确认问题。
- `uni/prd/PRD.md` 是新完整 PRD；不覆盖现有 `uni/PRD.md`。
- 计划、库存、单位换算和风险阈值等确定性结论不交给 LLM。
