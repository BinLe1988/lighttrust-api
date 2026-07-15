# Bedrock 对账插件部署指南

本文说明方案 A 的首个平台接入：LightTrust 通过本机/容器的 AWS 基础身份调用 STS，携带独立 External ID 扮演目标 AWS 账户中的专用 IAM Role；对账 Role 只读 Bedrock 调用日志、CUR、Cost Explorer 与 CloudWatch 指标。

## 1. 数据准备

1. 在每个目标 Region 启用 Amazon Bedrock model invocation logging，输出到 CloudWatch Logs 或 S3。建议日志至少保留 35 天。
2. 创建 CUR 2.0 Data Export，包含资源 ID，并通过 Glue/Athena 暴露为表。对账查询依赖 `line_item_usage_start_date`、Region、产品/用量类型、资源 ID、用量和未混合成本列。
3. 开启 Cost Explorer；若需要核对抵扣、退款和税费，配置中启用 Cost Explorer。
4. 确认 LightTrust 的 Bedrock 渠道已按 AWS Region 建立，记录渠道 ID。

参考：

- [Bedrock model invocation logging](https://docs.aws.amazon.com/bedrock/latest/userguide/model-invocation-logging.html)
- [CUR 2.0 table dictionary](https://docs.aws.amazon.com/cur/latest/userguide/table-dictionary-cur2.html)
- [Bedrock runtime metrics](https://docs.aws.amazon.com/bedrock/latest/userguide/monitoring-runtime-metrics.html)

## 2. 建立独立 IAM Role

目标 AWS 账户中新建 Role，例如 `LightTrustBedrockReconciliationRole`。信任策略中的 Principal 替换为 LightTrust 运行身份，External ID 使用独立随机值，并与对账配置一致。

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::<LIGHTTRUST_ACCOUNT_ID>:role/<LIGHTTRUST_RUNTIME_ROLE>"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "<UNIQUE_EXTERNAL_ID>"
        }
      }
    }
  ]
}
```

LightTrust 运行身份还需允许对上述 Role 执行 `sts:AssumeRole`。专用对账 Role 按实际资源范围授予：

- CloudWatch Logs：`logs:StartQuery`、`logs:GetQueryResults`、`logs:StopQuery`。
- 调用日志/CUR/Athena 输出 S3：`s3:ListBucket`、`s3:GetBucketLocation`、`s3:GetObject`；Athena 输出路径还需 `s3:PutObject`。
- Athena：`athena:StartQueryExecution`、`athena:GetQueryExecution`、`athena:GetQueryResults`、`athena:GetWorkGroup`。
- Glue Data Catalog：`glue:GetDatabase`、`glue:GetTable`、`glue:GetPartitions`。
- Cost Explorer：`ce:GetCostAndUsage`。
- CloudWatch：`cloudwatch:GetMetricData`。
- 如果 S3、日志或 Athena 输出使用客户托管 KMS Key，增加相应 `kms:Decrypt` / `kms:GenerateDataKey` 权限。

生产环境应把 bucket、prefix、log group、database、table、workgroup 和 KMS Key 收紧到实际资源。

## 3. 创建对账配置

使用有 `reconcile.sensitive_write` 权限的管理员进入“管理 → 对账”：

1. 填写 AWS 账户 ID、Role ARN 和 External ID。External ID 仅写入，不通过 API 回显。
2. 选择 CloudWatch Logs 或 S3 调用日志来源并填写对应位置。
3. 填写 Athena database、table、workgroup 与 `s3://...` 输出位置。
4. 填写 Region 列表和 Region 到 AWS Bedrock 渠道 ID 的 JSON 映射。
5. 保存后先运行“诊断访问权限”，分别验证 AssumeRole、调用日志、Athena/CUR、Cost Explorer 和 CloudWatch。
6. 先用短时间窗口手工执行，对比请求、每日成本、账户支出三层结果。

## 4. 调度与上线

定时任务默认关闭。完成权限诊断和至少一次人工回填后设置：

```text
BEDROCK_RECONCILIATION_TASK_ENABLED=true
BEDROCK_RECONCILIATION_TASK_INTERVAL_MINUTES=360
```

间隔小于 15 分钟会被提升到 15 分钟。每个配置还必须设置 `enabled=true`。建议先保留默认 3 天回溯和 30 分钟成熟延迟，观察 CUR 到达时延后再调整。

## 5. 权限与安全边界

- `reconcile.read`：查看配置（不含 External ID）、运行和结果。
- `reconcile.operate`：访问诊断、手工运行和重试。
- `reconcile.sensitive_write`：创建、修改、删除配置。
- `reconcile.export`：导出 CSV；单次最多 10,000 行。
- AWS SDK 错误返回经过脱敏，不向界面暴露凭据或上游错误正文。
- 多实例调度由系统任务租约去重；结果写入采用业务唯一键保证重试幂等。
