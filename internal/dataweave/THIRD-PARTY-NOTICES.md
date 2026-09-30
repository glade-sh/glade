# DataWeave engine distribution

The owned Glade adapter uses unmodified MuleSoft DataWeave engine 2.12.2 and its
locked dependencies. The exact artifact URLs and SHA256 values are in
artifacts.json. Engine archives retain their original embedded license and
notice files. Installation also extracts those notices into notices/.

The MuleSoft engine, Scala, parboiled, Jackson XML implementations and Commons IO
are under Apache License 2.0. Stax2 uses the BSD 2-Clause license. Jakarta
Activation and Angus Activation use Eclipse Distribution License 1.0.

Jakarta Mail API 2.1.5 and Angus Mail 2.0.5 are distributed here under Eclipse
Public License 2.0. Their exact corresponding source archives are included in
sources/ alongside the engine distribution. Those archives are the preferred
form of their source code and may be used and redistributed under their included
licenses. Their notices also describe the GPL2 with Classpath Exception option;
this distribution uses EPL2. Some included content has separate EDL terms, which
does not make the entire mail libraries EDL-licensed.

This adapter does not modify or copy the implementation of those components.
Glade's own code remains under the Glade repository license. A Java runtime is
provisioned separately and its license and notices must accompany its own
redistribution.

Primary terms:
- https://www.apache.org/licenses/LICENSE-2.0
- https://www.eclipse.org/legal/epl-2.0/
- https://www.eclipse.org/org/documents/edl-v10.php
