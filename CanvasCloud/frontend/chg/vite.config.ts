import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      // 主API代理
      '^/lwh': {
        target: 'localhost:8001', // 关键修改：改为localhost
        changeOrigin: true,
        // 保留路径前缀
      },
      
      // Swagger文档代理
      '^/swagger': {
        target: 'http://localhost:8001', // 改为localhost
        changeOrigin: true,
        rewrite: path => path.replace(/^\/swagger/, '')
      },
      
      // WebSocket代理
      '^/ws': {
        target: 'ws://localhost:8001', // 改为localhost
        ws: true,
        changeOrigin: true
      },
      
      // 文件上传代理
      '^/file': {
        target: 'http://localhost:8001', // 改为localhost
        changeOrigin: true,
        rewrite: path => path.replace(/^\/file/, '/lwh/file') // 添加正确的路径重写
      }
    }
  }
})