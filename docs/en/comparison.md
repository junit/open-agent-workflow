# Design Comparison

OAW separates three concerns that are often mixed together:

| Concern | OAW owner |
| --- | --- |
| Engineering method and defaults | readable Markdown Policy and Profile |
| Skill procedure | the independently installed Skill |
| Physical execution and permissions | Agent Host |

The installer is not a workflow runner. It distributes the Policy Set and
target-native activation routers. Profile inspection is advisory and cannot
select a method or prove a Skill's contents.

Optional machine evidence is deliberately additive. It can make a claim more
precise, but missing evidence cannot remove a Profile or stop the model from
following readable rules.

This design favors a small, understandable rule surface over a second
workflow implementation. It is based on the project's dogfood experience and
is not a claim about every other agent product.
