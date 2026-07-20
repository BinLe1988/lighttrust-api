#!/usr/bin/env python3
"""
lighttrust-api 小型压测脚本
生成 1000 条多样化请求并发发送，覆盖 chat completion、流式、错误场景。

用法:
  # 从后台 "令牌" 页面创建 API Token，获取 User ID
  python3 loadtest.py --base http://localhost:3000 --token sk-xxx --user-id 1

依赖: pip install aiohttp
"""
import argparse, asyncio, json, random, time, sys
from urllib.parse import urljoin

try:
    import aiohttp
except ImportError:
    print("pip install aiohttp"); sys.exit(1)

# ── 请求模板池 ──────────────────────────────────────

SHORT_CONVO = ["Hi", "Hello", "What is 2+2?", "Who are you?", "Tell me a joke",
    "What's the weather?", "Define AI", "Capital of France?",
    "How are you?", "Say hello in Spanish", "What is Python?",
    "Is water wet?", "What is 42?", "Name a color", "Count to 5",
    "What is time?", "Say goodbye", "Thank you", "What is gravity?"]

SYSTEM_PROMPTS = [
    "You are a helpful assistant.",
    "You are a poet. Reply in rhyme.",
    "You are a math tutor. Explain step by step.",
    "You speak like a pirate.",
    "You are a cynical AI. Be sarcastic.",
    "Answer in Chinese.",
    "You are a code reviewer. Be strict.",
    "You are a historian. Give dates and context.",
]

USER_QUESTIONS = [
    "Explain gravity",
    "Write a poem about AI",
    "What is recursion?",
    "How do rockets work?",
    "Tell me about Mars",
    "What is the meaning of life?",
    "Compare cats and dogs",
    "Write a haiku about coding",
    "How does the internet work?",
    "What is machine learning?",
    "Tell me a story about AI",
    "Explain neural networks",
    "How do transformers work?",
    "Write a short story about robots",
    "Describe the solar system",
]

LONG_TEXT = "The quick brown fox jumps over the lazy dog. " * 50
LONG_TOPICS = [
    f"Summarize this: {LONG_TEXT}",
    f"Explain quantum computing in detail. {'What are qubits?' * 30}",
    "Write a Python function to sort a list of dicts by multiple keys with type hints and docstrings. " * 5,
]

MID_TURN = ["Great, now expand on that.", "Can you give me an example?",
            "Why is that important?", "Translate that to Chinese.",
            "Simplify that for a 5 year old."]

# ── 请求生成 ───────────────────────────────────────

def pick(rng, seq):
    return seq[rng.randint(0, len(seq)-1)]

def make_chat(scenario: int, idx: int, rng: random.Random):
    model = rng.choice(["deepseek-chat"] * 4 + ["deepseek-reasoner"])

    if scenario == 0:  # 简短单轮
        msgs = [{"role": "user", "content": pick(rng, SHORT_CONVO)}]
        tag = "short"

    elif scenario == 1:  # 系统提示
        msgs = [{"role": "system", "content": pick(rng, SYSTEM_PROMPTS)},
                {"role": "user", "content": pick(rng, USER_QUESTIONS)}]
        tag = "system"

    elif scenario == 2:  # 多轮对话
        msgs = [{"role": "user", "content": pick(rng, USER_QUESTIONS)}]
        for _ in range(rng.randint(1, 3)):
            msgs.append({"role": "assistant", "content": "Good question. Let me explain."})
            msgs.append({"role": "user", "content": pick(rng, MID_TURN)})
        tag = "multi-turn"

    elif scenario == 3:  # 长文本
        msgs = [{"role": "user", "content": pick(rng, LONG_TOPICS)}]
        tag = "long"

    elif scenario == 4:  # 各种参数组合
        msgs = [{"role": "user", "content": pick(rng, USER_QUESTIONS)}]
        params = dict(
            temperature=rng.choice([0.1, 0.5, 0.7, 0.9, 1.5]),
            top_p=rng.choice([0.5, 0.8, 0.9, 1.0]),
            max_tokens=rng.choice([50, 100, 200, 500, 1024, 2048]),
        )
        tag = "params"
    else:
        msgs = [{"role": "user", "content": "Hello"}]
        params = {}
        tag = "default"

    payload = dict(model=model, messages=msgs)
    if scenario == 4:
        payload.update(params)
    return payload, tag

def bad_payloads():
    """返回 (payload_factory, scenario_tag) 列表"""
    return [
        (lambda: {"model": "deepseek-chat"}, "err-no-msg"),
        (lambda: {"model": "deepseek-chat", "messages": []}, "err-empty-msg"),
        (lambda: {"model": "deepseek-chat",
                   "messages": [{"role": "user", "content": 123}]}, "err-bad-type"),
        (lambda: {"model": "non-existent-model-12345",
                   "messages": [{"role": "user", "content": "Hi"}]}, "err-bad-model"),
    ]

# ── 请求发送 ───────────────────────────────────────

async def send_one(session, base, token, uid, json_payload, tag,
                    sem, stats_list, stream=False):
    async with sem:
        headers = {
            "Authorization": f"Bearer {token}",
            "New-Api-User": str(uid),
            "Content-Type": "application/json",
        }
        url = urljoin(base, "/v1/chat/completions")
        t0 = time.monotonic()
        try:
            async with session.post(url, json=json_payload, headers=headers,
                                    timeout=aiohttp.ClientTimeout(total=120)) as resp:
                status = resp.status
                if stream:
                    async for _ in resp.content:
                        pass
                else:
                    await resp.read()
                latency = time.monotonic() - t0
                stats_list.append((tag, status < 500, latency, status))
        except Exception as e:
            latency = time.monotonic() - t0
            stats_list.append((tag, False, latency, 0))

# ── 主流程 ─────────────────────────────────────────

async def main():
    ap = argparse.ArgumentParser(description="lighttrust-api load test")
    ap.add_argument("--base", default="http://localhost:3000")
    ap.add_argument("--token", required=True, help="API Token (从后台 令牌 页面创建)")
    ap.add_argument("--user-id", type=int, default=1, help="User ID (后台可见)")
    ap.add_argument("-c", "--concurrency", type=int, default=10)
    ap.add_argument("--stream-prob", type=float, default=0.3,
                    help="流式请求比例 0-1")
    ap.add_argument("--bad-prob", type=float, default=0.05,
                    help="错误请求比例 0-1")
    args = ap.parse_args()

    print(f"lighttrust-api 压测")
    print(f"  目标:       {args.base}")
    print(f"  并发:       {args.concurrency}")
    print(f"  请求总数:   1000")
    print(f"  流式比例:   {args.stream_prob:.0%}")
    print(f"  错误比例:   {args.bad_prob:.0%}")
    print()

    rng = random.Random(42)

    # 生成 1000 条请求
    tasks = []  # (json, tag, stream)
    for i in range(1000):
        if rng.random() < args.bad_prob:
            factory, tag = rng.choice(bad_payloads())
            tasks.append((factory(), tag, False))
        else:
            scenario = rng.choices(
                population=[0, 1, 2, 3, 4],
                weights=[30, 25, 15, 10, 20],
                k=1)[0]
            payload, tag = make_chat(scenario, i, rng)
            stream = (rng.random() < args.stream_prob) or (scenario == 3 and rng.random() < 0.5)
            if stream:
                payload["stream"] = True
                tag = "stream"
            tasks.append((payload, tag, stream))

    stats_list = []
    sem = asyncio.Semaphore(args.concurrency)
    connector = aiohttp.TCPConnector(limit=args.concurrency, force_close=True)

    async with aiohttp.ClientSession(connector=connector) as session:
        coros = [send_one(session, args.base, args.token, args.user_id,
                          payload, tag, sem, stats_list, stream)
                 for payload, tag, stream in tasks]

        batch_size = 100
        for i in range(0, len(coros), batch_size):
            batch = coros[i:i+batch_size]
            await asyncio.gather(*batch)
            ok = sum(1 for _, o, _, _ in stats_list if o)
            err = len(stats_list) - ok
            sys.stdout.write(f"\r  进度: {len(stats_list):4d}/1000  "
                             f"成功={ok}  失败={err}")
            sys.stdout.flush()
        print()

    # ── 报告 ──
    by_scenario = {}
    for tag, ok, lat, status in stats_list:
        by_scenario.setdefault(tag, {"ok": 0, "err": 0, "n": 0, "lats": []})
        s = by_scenario[tag]
        s["n"] += 1
        if ok:
            s["ok"] += 1
        else:
            s["err"] += 1
        s["lats"].append(lat)

    all_lats = [lat for _, _, lat, _ in stats_list]
    print()
    print("=" * 55)
    print("  结果汇总")
    print("=" * 55)
    total_ok = sum(1 for _, o, _, _ in stats_list if o)
    total_err = len(stats_list) - total_ok
    print(f"  总请求:  {len(stats_list)}")
    print(f"  成功:    {total_ok}")
    print(f"  失败:    {total_err}")
    if all_lats:
        all_lats.sort()
        p50 = all_lats[len(all_lats)//2]
        p95 = all_lats[int(len(all_lats)*0.95)]
        p99 = all_lats[int(len(all_lats)*0.99)]
        avg = sum(all_lats) / len(all_lats)
        print(f"  延迟 p50:  {p50:.3f}s")
        print(f"  延迟 p95:  {p95:.3f}s")
        print(f"  延迟 p99:  {p99:.3f}s")
        print(f"  延迟 avg:  {avg:.3f}s")
    print()
    print("=" * 55)
    print("  场景明细")
    print("=" * 55)
    for tag in sorted(by_scenario):
        d = by_scenario[tag]
        ok_rate = d["ok"] / d["n"] * 100 if d["n"] else 0
        avg_lat = sum(d["lats"]) / len(d["lats"]) if d["lats"] else 0
        print(f"  {tag:14s}  n={d['n']:4d}  ok={d['ok']:4d}  "
              f"err={d['err']:4d}  {ok_rate:5.1f}%  "
              f"avg={avg_lat:.3f}s")

if __name__ == "__main__":
    asyncio.run(main())
