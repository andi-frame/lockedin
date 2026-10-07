// A pact has exactly two members and each keeps one line colour everywhere (calendar, ledger,
// avatars). The slot comes from the sorted ids so both people see the same colours whatever
// order the API lists the members in.
export type MemberSlot = 0 | 1;

export function memberSlot(memberId: string, memberIds: readonly string[]): MemberSlot {
  const index = [...memberIds].sort().indexOf(memberId);
  return index === 1 ? 1 : 0;
}
