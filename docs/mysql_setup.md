# 涅凰智农本地 MySQL 初始化

## 1. 结论

不要修改 `ShoreOS/services/shoreos-server/scripts/mysql_shoreos.sh`，也不要给 `shoreos_agent` 扩权。

原因：

- `shoreos_agent` 是 ShoreOS 项目的受限账号。
- 它只能操作 `shoreos` 库，不能创建 `nehuang_db` 是正常设计。
- LOT/涅凰平台应该使用独立数据库和独立账号，避免项目数据混在一起。

## 2. 初始化专用库和账号

在项目目录执行：

```bash
cd /Users/shore/Knowledge/nehuang-iot-server
scripts/init_mysql_nehuang.sh
```

脚本会：

- 创建 `nehuang_db`
- 创建 `nehuang_agent`
- 授权 `nehuang_agent` 操作 `nehuang_db`
- 写入本地凭证文件 `configs/mysql/nehuang_agent.cnf`

脚本会提示输入本机 MySQL 管理员密码，也会提示输入 `nehuang_agent` 的新密码。

## 3. 导入表结构

```bash
cd /Users/shore/Knowledge/nehuang-iot-server
scripts/mysql_nehuang.sh < schema/mysql/001_nehuang_v1.sql
scripts/mysql_nehuang.sh < schema/mysql/002_device_node_point_model.sql
```

验证：

```bash
scripts/mysql_nehuang.sh -e "SHOW TABLES;"
```

## 4. 日常连接

```bash
cd /Users/shore/Knowledge/nehuang-iot-server
scripts/mysql_nehuang.sh
```

或者执行 SQL：

```bash
scripts/mysql_nehuang.sh -e "SELECT DATABASE();"
```

## 5. 如已有账号

如果你已经手工创建了账号，只需要复制示例配置：

```bash
cp configs/mysql/nehuang_agent.cnf.example configs/mysql/nehuang_agent.cnf
chmod 600 configs/mysql/nehuang_agent.cnf
```

然后把里面的 `user/password/database` 改成本地实际值。
