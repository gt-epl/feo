def main(args):
  res = 0
  res += args.get("payload")
  res += 2

  return {"payload": res}
