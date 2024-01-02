def main(args):

  res = 0
  res += args.get("payload2")
  res += args.get("payload3")
  res += 4

  return {"payload4": res}
