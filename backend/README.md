# Cải tiến hiệu năng cho chương trình:  
Tất cả các test đều được thực hiện bằng autocannon với 1000 concurrent connections trong thời gian 30 giây:  
`autocannon -d 30 -c 1000`
- Hiện tại chỉ test API trả về đường dẫn gốc với 1 link hợp lệ trong db, chưa benchmark API tạo link rút gọn.
## Kết quả benchmark:

### Code gốc:

| Stat    | 2.5%   | 50%    | 97.5%  | 99%    | Avg       | Stdev    | Max     |
|---------|--------|--------|--------|--------|-----------|----------|---------|
| Latency | 361 ms | 392 ms | 499 ms | 831 ms | 407.79 ms | 62.13 ms | 1102 ms |

| Stat      | 1%     | 2.5%   | 50%    | 97.5%  | Avg     | Stdev   | Min    |
|-----------|--------|--------|--------|--------|---------|---------|--------|
| Req/sec   | 1024   | 1024   | 2505   | 2769   | 2441.97 | 326.3   | 1024   |
| Bytes/sec | 244 KB | 244 KB | 596 KB | 659 KB | 581 KB  | 77.6 KB | 244 KB |

### Thay `express` bằng `hyper-express`:
| Stat    | 2.5%   | 50%    | 97.5%  | 99%    | Avg       | Stdev    | Max    |
|---------|--------|--------|--------|--------|-----------|----------|--------|
| Latency | 173 ms | 157 ms | 227 ms | 235 ms | 172.12 ms | 43.06 ms | 653 ms |

| Stat      | 1%     | 2.5%   | 50%    | 97.5%  | Avg    | Stdev   | Min    |
|-----------|--------|--------|--------|--------|--------|---------|--------|
| Req/sec   | 3129   | 3129   | 5875   | 6279   | 5821.7 | 553.79  | 3128   |
| Bytes/sec | 272 KB | 272 KB | 511 KB | 546 KB | 506 KB | 48.2 KB | 272 KB |

### Thêm cache để tránh query database nhiều lần:

| Stat    | 2.5%  | 50%   | 97.5% | 99%   | Avg      | Stdev    | Max    |
|---------|-------|-------|-------|-------|----------|----------|--------|
| Latency | 42 ms | 49 ms | 60 ms | 71 ms | 50.39 ms | 25.15 ms | 699 ms |

| Stat      | 1%     | 2.5%   | 50%     | 97.5%   | Avg      | Stdev   | Min    |
|-----------|--------|--------|---------|---------|----------|---------|--------|
| Req/sec   | 5507   | 5507   | 20207   | 21743   | 19733.14 | 2749.39 | 5505   |
| Bytes/sec | 479 KB | 479 KB | 1.76 MB | 1.89 MB | 1.72 MB  | 239 KB  | 479 KB |   

### Spawn node cluster với cache riêng biệt:

| Stat    | 2.5%  | 50%   | 97.5% | 99%   | Avg      | Stdev    | Max    |
|---------|-------|-------|-------|-------|----------|----------|--------|
| Latency | 41 ms | 48 ms | 61 ms | 67 ms | 50.01 ms | 25.62 ms | 701 ms |

| Stat      | 1%     | 2.5%   | 50%     | 97.5%   | Avg     | Stdev   | Min    |
|-----------|--------|--------|---------|---------|---------|---------|--------|
| Req/sec   | 5679   | 5679   | 20319   | 21999   | 19873.8 | 2741.58 | 5676   |
| Bytes/sec | 494 KB | 494 KB | 1.77 MB | 1.91 MB | 1.73 MB | 239 KB  | 494 KB |