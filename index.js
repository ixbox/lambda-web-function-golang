import { resolve } from 'node:path';

// spawn で子プロセスにすると Node.js が常駐し、SIGTERM の中継やメモリ消費が
// 余計に発生する。execve で同一 PID のまま Go バイナリに置き換える。
// その代わり、ランタイムが行う ready 通知は Go 側で肩代わりする（main.go の reportReady）。
process.execve(resolve(import.meta.dirname, 'bootstrap'), ['bootstrap'], process.env);
