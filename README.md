# caseurl2md

cURL请求到树状JSON转换工具

## 功能介绍

caseurl2md 是一个命令行工具，能够将cURL命令转换为精简的树状JSON结构。该工具支持：

1. 解析cURL命令
2. 执行HTTP请求
3. 从JSON响应中抽取树状结构
4. 输出仅包含 `case_title` 和 `children` 的精简JSON

## 安装

### 从源码编译

```bash
git clone <repository-url>
cd caseurl2md
go build -o caseurl2md .
```

### 使用

编译成功后，将 `caseurl2md` 可执行文件放到你的 PATH 中：

```bash
sudo mv caseurl2md /usr/local/bin/
```

## 使用方法

### 1. 直接使用cURL命令

```bash
./caseurl2md --from-curl 'curl "http://api.example.com/data" -H "Authorization: Bearer token"'
```

### 2. 从文件读取cURL

```bash
echo 'curl "http://api.example.com/data" -H "Authorization: Bearer token"' > curl.txt
./caseurl2md --curl-file curl.txt --out result.json
```

### 3. 手动指定参数

```bash
./caseurl2md --url "http://api.example.com/data" \
             --header "Content-Type: application/json" \
             --header "Authorization: Bearer token" \
             --method GET
```

### 4. 从stdin读取

```bash
echo 'curl "http://api.example.com/data"' | ./caseurl2md
```

## 命令行参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `--from-curl` | 直接从命令行接收cURL命令 | - |
| `--curl-file` | 从文件读取cURL命令 | - |
| `--url` | 请求URL（不使用cURL时必需） | - |
| `--method` | 请求方法 | `GET` |
| `--header` | 请求头，格式为'Key: Value'，可多次使用 | - |
| `--data` | 请求体数据 | - |
| `--out` | 输出文件路径（默认为output_{timestamp}.json） | - |
| `--title-key` | 节点内容字段候选键名，按优先级排序 | `[case_title,title,name,label]` |
| `--children-keys` | 子节点数组候选键名，按优先级排序 | `[children,nodes,sub_cases,items,data]` |
| `--timeout` | HTTP请求超时时间（秒） | `30` |
| `--verbose` | 显示详细日志 | `false` |

## 输出格式

工具输出的JSON格式统一为：

```json
{
  "case_title": "节点标题",
  "children": [
    {
      "case_title": "子节点标题",
      "children": []
    }
  ]
}
```

## 工作流程

1. **输入解析**：支持从 stdin、文件或命令行参数获取cURL命令或请求信息
2. **cURL解析**：解析cURL命令，提取URL、方法、请求头和请求体
3. **HTTP请求**：使用解析出的参数执行真实的HTTP请求
4. **响应校验**：检查响应状态码和JSON格式
5. **树抽取**：递归遍历JSON数据，抽取树状结构
6. **输出**：将结果保存为格式化的JSON文件

## 示例

### 简单示例

假设服务器返回：
```json
{
  "title": "项目A",
  "nodes": [
    {
      "name": "功能1",
      "items": []
    },
    {
      "name": "功能2",
      "items": [
        {
          "label": "子功能2.1",
          "elements": []
        }
      ]
    }
  ]
}
```

使用命令：
```bash
./caseurl2md --url "http://api.example.com/data" \
             --title-key "title,name,label" \
             --children-keys "nodes,items,elements"
```

输出结果：
```json
{
  "case_title": "项目A",
  "children": [
    {
      "case_title": "功能1",
      "children": []
    },
    {
      "case_title": "功能2",
      "children": [
        {
          "case_title": "子功能2.1",
          "children": []
        }
      ]
    }
  ]
}
```

## 错误处理

工具提供了详细的错误信息：

- **cURL解析失败**：检查cURL语法
- **网络错误**：检查网络连接和URL
- **非2xx状态码**：服务器返回错误，检查请求参数
- **非JSON响应**：服务器响应不是有效JSON
- **未找到树结构**：响应中不符合抽取规则的树状数据

## 开发

### 项目结构

```
caseurl2md/
├── main.go                 # 主入口
├── internal/
│   ├── cli/               # CLI参数处理
│   ├── config/            # 配置结构
│   ├── parser/            # cURL解析器
│   ├── http/              # HTTP执行器
│   ├── validator/         # 响应校验器
│   ├── extractor/         # 树抽取器
│   └── processor/         # 主处理器
├── test/                  # 测试相关
└── docs/                  # 文档
```

### 运行测试

```bash
# 启动测试服务器
cd test
go run test_server.go

# 在另一个终端运行测试
./caseurl2md --url "http://localhost:8080/simple" --verbose
```

## License

[MIT License](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request！