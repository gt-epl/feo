def main(args):
  res = 0
  res += args.get("payload")
  res += 3

  return {"payload": res}
