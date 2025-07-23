import { generateService } from '@umijs/openapi'

generateService({
  requestLibPath: "import request from '@/request'",
  schemaPath:
    'http://localhost:8001/swagger/doc.json' /* 生成请求函数的参考地址，为后端的swagger */,
  serversPath: './src',
})
