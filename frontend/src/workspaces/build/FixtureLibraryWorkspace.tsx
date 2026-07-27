import { Lightbulb } from "lucide-react";

import ComingSoon from "../ComingSoon";

export default function FixtureLibraryWorkspace() {
  return (
    <ComingSoon
      title="Fixture Library"
      icon={Lightbulb}
      description="The Fixture Library browser isn't wired into the desktop app yet."
      cliHint="Use golc fixture from the command line to inspect fixture definitions in the meantime."
    />
  );
}
