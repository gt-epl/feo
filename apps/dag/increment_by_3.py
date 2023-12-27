def main(args):
  input_list = args.get("inputs")

  res = 0
  for input_payload in input_list:
    res += input_payload.get("payload")
  res += 3

  return {"payload": res}