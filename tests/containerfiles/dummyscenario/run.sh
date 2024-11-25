[ -z $END ] && echo '$END variable not exported' && exit 1
[ -z $EXIT_STATUS ] && echo '$EXIT_STATUS variable not exported' && exit 1
for i in $(seq 0 $END); do
  echo "Release the krkn $i"
  sleep 1
done
echo "EXITING $EXIT_STATUS"
exit $EXIT_STATUS