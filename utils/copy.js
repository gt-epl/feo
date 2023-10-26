async function main (params) {
  const sleep = (delay) => new Promise((resolve) => setTimeout(resolve, delay))

  //async function Sleep() {
  //  await sleep(1000);
  //}

  await sleep(100);

  var input = params.input || ''
  return { payload: input }
}
