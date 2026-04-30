import { Component, type ReactNode } from "react";
type Props = { children: ReactNode };
type State = { error: Error | null };
export class WidgetErrorBoundary extends Component<Props, State> {
  state: State = { error: null };
  static getDerivedStateFromError(e: Error) { return { error: e }; }
  render() {
    if (this.state.error) return <div className="widget-error">Widget error: {this.state.error.message}</div>;
    return this.props.children;
  }
}
